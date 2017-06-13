package gotcp

/*
	1. 启动读、写、分发进程
	2. 如何通知其关闭
	3. 定时心跳问题?
 */

import (
	"sync"
	"net"
	"time"
	"sync/atomic"
)

type ConnExOptions struct {
	SendChanLimit int
	RecvChanLimit int
	Nodelay       bool //TODO
}

type ConnEx struct {
	conn            *net.TCPConn
	packetSendChan  chan Packet
	packetRecvChan  chan Packet
	CloseChan       chan struct{}
	closeFlag       int32
	closeOnce       sync.Once

	//outter interface input
	cbs             ConnExCbs
	wg              *sync.WaitGroup
	proto           Protocol
	opts            ConnExOptions
	extraData       interface{}

}

type ConnExCbs interface {
	OnConnect(*ConnEx) bool
	OnMessage(*ConnEx, Packet) bool
	OnClose(*ConnEx)
}

func (c *ConnEx) IsClosed() bool{
	return atomic.LoadInt32(&c.closeFlag) == 1
}

func NewConnEx(conn *net.TCPConn, cbs ConnExCbs, wg *sync.WaitGroup, opts ConnExOptions, proto Protocol) *ConnEx{
	return &ConnEx{
		conn: conn,
		CloseChan:make(chan struct{}),
		opts:opts,
		packetRecvChan:make(chan Packet, opts.RecvChanLimit),
		packetSendChan:make(chan Packet, opts.SendChanLimit),
		wg:wg,
		cbs:cbs,
		proto:proto,
	}
}

func (c *ConnEx) Close(){
	c.closeOnce.Do( func() {
		atomic.StoreInt32(&c.closeFlag, 1)
		close(c.CloseChan)
		close(c.packetRecvChan)
		close(c.packetSendChan)
		c.conn.Close()
		c.cbs.OnClose(c)
	})
}

func (c *ConnEx) Run(){
	if(!c.cbs.OnConnect(c)){
		return
	}
	asyncDo(c.handleLoop,c.wg)
	asyncDo(c.readLoop,c.wg)
	asyncDo(c.writeLoop,c.wg)
}

func (c *ConnEx) readLoop(){
	defer func() {
		recover()
		c.Close()
	}()
	for {
		select {
		case <-c.CloseChan:
			return
		default:
		}
		//TODO : check connection closed
		p, err := c.proto.ReadPacket(c.conn)
		if(err != nil){
			return
		}
		c.packetRecvChan <- p
	}

}

func (c *ConnEx) writeLoop(){
	defer func() {
		recover()
		c.Close()
	}()

	for {
		select {
		case <- c.CloseChan:
			return
		case p:= <-c.packetSendChan:
			if(c.IsClosed()){
				return
			}
			if _, err := c.conn.Write(p.Serialize()); err != nil{
				return
			}
		}
	}
}

func (c *ConnEx) handleLoop(){
	defer func() {
		recover()
		c.Close()
	}()

	for {
		select {
		case <-c.CloseChan:
			return
		case p := <-c.packetRecvChan:
			if c.IsClosed(){
				return
			}
			if !c.cbs.OnMessage(c, p){
				return
			}
		}
	}
}

func (c *ConnEx) AsyncWritePacket(packet *Packet, timeout time.Duration) error{
	if(c.IsClosed()){
		return ErrConnExClosed
	}

	if timeout = 0{
		select {
		case c.packetSendChan <- *packet:
			return nil
		case <- c.CloseChan:
			return ErrConnExClosed
		default:
			return ErrConnExWriteBlocking
		}
	}else{
		select {
		case c.packetSendChan <- *packet:
			return nil
		case <- c.CloseChan:
			return ErrConnExClosed
		case <-time.After(timeout):
			return ErrConnExWriteBlocking
		}
	}
}
