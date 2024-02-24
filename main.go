package main

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/quic-go/quic-go"
)

const addr = "localhost:42424"

type emptyCtx struct{}

func (emptyCtx) Deadline() (deadline time.Time, ok bool) {
	return
}

func (emptyCtx) Done() <-chan struct{} {
	return nil
}

func (emptyCtx) Err() error {
	return nil
}

func (emptyCtx) Value(key any) any {
	return nil
}

func (s *Server) go_con_quic(con quic.Connection) {
	fmt.Println("Got connection to client!")
	str, err := con.AcceptStream(con.Context())
	if err != nil {
		panic(err.Error())
	}
	eval_stream(s.Commands, str, str)
}

func (s *Server) Serve() {
	keyFile := s.KeyFile
	certFile := s.CertFile
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	tlscfg := tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{cert},
	}

	listener, err := quic.ListenAddr(addr, &tlscfg, nil)
	if err != nil {
		panic(err.Error())
		return
	}

	defer listener.Close()
	go func() {
		<-s.End
		listener.Close()
	}()
	x := emptyCtx{}
	for {
		fmt.Println("Listening for connection")
		con, err := listener.Accept(&x)
		if err != nil {
			panic(err.Error())
		}
		fmt.Println("Got connection")
		go s.go_con_quic(con)

	}

}

type Client struct {
	con quic.Connection
}

func (cli *Client) OpenStream() (quic.Stream, error) {
	return cli.con.OpenStream()
}

func new_client() Client {
	ctx := emptyCtx{}

	tlscfg := tls.Config{InsecureSkipVerify: true}
	quiccfg := quic.Config{}

	con, err := quic.DialAddr(&ctx, addr, &tlscfg, &quiccfg)

	if err != nil {
		panic(err.Error())
	}

	return Client{con}
}

func main() {
	server := ServerNew()
	server.Serve()
}
