package command

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/client"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/version"
)

func (rt *Runtime) Client() (*client.Client, error) {
	if rt.client != nil {
		return rt.client, nil
	}
	c, err := client.New(client.Options{
		SocketPath: rt.Socket,
		Timeout:    rt.Timeout,
		ClientName: "hdkt",
		UserAgent:  "hdkt/" + version.Get().Version,
	})
	if err != nil {
		return nil, err
	}
	rt.client = c
	return c, nil
}

func (rt *Runtime) Ping(ctx context.Context) error {
	c, err := client.New(client.Options{
		SocketPath: rt.Socket,
		Timeout:    min(rt.Timeout, 3*time.Second),
		ClientName: "hdkt",
	})
	if err != nil {
		return err
	}
	defer c.Close()
	_, err = c.Ping(ctx)
	return err
}

func (rt *Runtime) Hello(ctx context.Context) (*protocol.HelloResponse, error) {
	c, err := rt.Client()
	if err != nil {
		return nil, err
	}
	v := version.Get()
	return c.Hello(ctx, protocol.HelloRequest{
		Client:        "hdkt",
		ClientVersion: v.Version,
		ProtocolMin:   protocol.SupportedVersions.Min,
		ProtocolMax:   protocol.SupportedVersions.Max,
		Features:      protocol.AllFeatures,
	})
}

func (rt *Runtime) Stdin() io.Reader {
	if rt.StdinReader == nil {
		return os.Stdin
	}
	return rt.StdinReader
}

func (rt *Runtime) printJSON(v any) error {
	return newJSONEncoder(rt.Stdout).Encode(v)
}
