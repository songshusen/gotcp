package gotcp

/*
	1. 支持断掉后的重连
	2.
 */

import (
	"net"
	"log"
	"sync"
	"time"
)

type ClientOptions struct {
	ReconnectInterval int //0 :表示不重新连接
	Addr              string
	ConnectTimeout    time.Duration
	Cbs               ConnExCbs
	ConnOptions       ConnExOptions
}

type Client struct{
	//conn            *net.TCPConn
	proto           Protocol
	exitChan        chan struct{}
	options         ClientOptions
	conn            *ConnEx

	//other
	tcpAddr         *net.TCPAddr
	wg              *sync.WaitGroup
	exitCh          chan struct{}
}

func NewClient(options *ClientOptions, proto Protocol) *Client {
	return &Client{
		options:*options,
		wg : &sync.WaitGroup{},
		exitCh:make(chan struct{}),
		proto : proto,
	}
}

/*
	反复重连的问题
 */
func (c *Client) Start() error{

	//connect to server
	conn, err := c.dial()
	if (err != nil && c.options.ReconnectInterval == 0){
		return err
	}
	log.Printf("dial ok\n")
	for {
		if((c.conn == nil || c.conn.IsClosed()) && (conn!=nil)){//TODO :条件待思考
			c.conn = NewConnEx(conn, c.options.Cbs, c.wg, c.options.ConnOptions, c.proto)
			c.wg.Add(1)
			go func() {
				c.conn.Run()
				c.wg.Done()
			}()
			//conn = nil
		}
		if(c.options.ReconnectInterval == 0){
			select {
			case <-c.exitCh:
				return nil
			default:
			}
		}else{
			select {
			case <-time.After(time.Second * time.Duration(c.options.ReconnectInterval)):
				if(c.conn == nil || c.conn.IsClosed()){
					conn, err = c.dial()
				}
			case <-c.exitCh:
				return nil
			}
		}
	}
}

func (c *Client) Stop(){
	if((c.conn!=nil) && !c.conn.IsClosed()){
		c.conn.CloseChan <- struct{}{}
	}
	close(c.exitCh)
	c.wg.Wait()
}

func (c *Client) dial() (*net.TCPConn, error) {
	addr, err := net.ResolveTCPAddr("tcp4", c.options.Addr)
	if (err != nil) {
		log.Printf("net.ResolveTCPAddr error : %s, address : %s\n", err.Error(), c.options.Addr)
		return nil, err
	}

	cn, err := net.DialTCP("tcp", nil, addr);
	if (err != nil) {
		log.Printf("net.DialTCP error : %s, address : %s\n", err.Error(), c.options.Addr)
		return nil, err;
	}
	return cn, err
}
