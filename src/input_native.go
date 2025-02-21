//go:build !js

package main

import (
	"net"
)

type TCPConnection struct {
	tcpConn *net.TCPConn
}

func (conn TCPConnection) Read(b []byte) (int, error) {
	return conn.tcpConn.Read(b)
}
func (conn TCPConnection) Write(b []byte) (int, error) {
	return conn.tcpConn.Write(b)
}
func (conn TCPConnection) Close() error {
	return conn.tcpConn.Close()
}

type TCPConnectionListener struct {
	server *net.TCPListener
}

func (conn TCPConnectionListener) Close() error {
	return conn.server.Close()
}

func (listener TCPConnectionListener) WaitForConnection() (NetConectionClient, error) {
	conn, err := listener.server.AcceptTCP()
	if err != nil {
		return nil, err
	}
	return &TCPConnection{
		tcpConn: conn,
	}, nil
}

func CreateNetConnectionListener(args ...string) (NetConnectionListener, error) {
	ln, err := net.Listen("tcp", ":"+args[0])
	if err != nil {
		return nil, err
	}
	return &TCPConnectionListener{
		server: ln.(*net.TCPListener),
	}, nil
}

func CreateNetConnection(args ...string) (NetConectionClient, error) {
	conn, err := net.Dial("tcp", args[0]+":"+args[1])
	if err != nil {
		return nil, err
	}
	return &TCPConnection{
		tcpConn: conn.(*net.TCPConn),
	}, nil
}
