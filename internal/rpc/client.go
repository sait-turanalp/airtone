// Package rpc is a minimal client for Snapcast's JSON-RPC control interface
// (newline-delimited JSON-RPC 2.0 over TCP, default port 1705). It powers the
// `status` command and, later, the TUI dashboard.
package rpc

import (
	"bufio"
	"encoding/json"
	"net"
	"time"
)

// DefaultAddr is the snapserver control endpoint.
const DefaultAddr = "127.0.0.1:1705"

// Stream is one audio stream and whether it is actively playing.
type Stream struct {
	ID     string
	Status string // "playing" | "idle"
}

// Client is one connected listener.
type Client struct {
	Name      string
	Connected bool
	Percent   int
	Muted     bool
}

// Status is the flattened view the UI needs.
type Status struct {
	Streams []Stream
	Clients []Client
}

// GetStatus queries Server.GetStatus and returns a flattened snapshot.
func GetStatus(addr string) (*Status, error) {
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	if _, err := conn.Write([]byte(`{"id":1,"jsonrpc":"2.0","method":"Server.GetStatus"}` + "\n")); err != nil {
		return nil, err
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			Server struct {
				Groups []struct {
					Clients []struct {
						Connected bool `json:"connected"`
						Host      struct {
							Name string `json:"name"`
						} `json:"host"`
						Config struct {
							Name   string `json:"name"`
							Volume struct {
								Muted   bool `json:"muted"`
								Percent int  `json:"percent"`
							} `json:"volume"`
						} `json:"config"`
					} `json:"clients"`
				} `json:"groups"`
				Streams []struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"streams"`
			} `json:"server"`
		} `json:"result"`
	}
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, err
	}

	s := &Status{}
	for _, st := range resp.Result.Server.Streams {
		s.Streams = append(s.Streams, Stream{ID: st.ID, Status: st.Status})
	}
	for _, g := range resp.Result.Server.Groups {
		for _, c := range g.Clients {
			name := c.Config.Name
			if name == "" {
				name = c.Host.Name
			}
			s.Clients = append(s.Clients, Client{
				Name:      name,
				Connected: c.Connected,
				Percent:   c.Config.Volume.Percent,
				Muted:     c.Config.Volume.Muted,
			})
		}
	}
	return s, nil
}
