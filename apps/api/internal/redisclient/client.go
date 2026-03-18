// Package redisclient provides a minimal RESP-based client for the go-redis clone.
// It speaks the Redis Serialization Protocol (RESP) directly over TCP.
package redisclient

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client is a connection to the go-redis server.
type Client struct {
	mu     sync.Mutex
	addr   string
	conn   net.Conn
	reader *bufio.Reader
}

// NewClient dials the go-redis server at addr (e.g. "localhost:6379").
func NewClient(addr string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("redisclient: dial %s: %w", addr, err)
	}
	return &Client{
		addr:   addr,
		conn:   conn,
		reader: bufio.NewReader(conn),
	}, nil
}

// Close closes the underlying TCP connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Set stores key=value. Returns nil on success.
func (c *Client) Set(key, value string) error {
	resp, err := c.do("SET", key, value)
	if err != nil {
		return err
	}
	if resp != "OK" {
		return fmt.Errorf("redisclient: SET unexpected response: %s", resp)
	}
	return nil
}

// Get retrieves the value for key. Returns ("", false, nil) if key does not exist.
func (c *Client) Get(key string) (string, bool, error) {
	val, err := c.doRaw("GET", key)
	if err != nil {
		return "", false, err
	}
	if val == nil {
		return "", false, nil
	}
	return *val, true, nil
}

// Del deletes one or more keys.
func (c *Client) Del(keys ...string) error {
	args := append([]string{"DEL"}, keys...)
	_, err := c.do(args...)
	return err
}

// Publish publishes message to channel. Returns the number of subscribers that received it.
func (c *Client) Publish(channel, message string) (int64, error) {
	resp, err := c.do("PUBLISH", channel, message)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(resp, 10, 64)
	if err != nil {
		return 0, nil // best effort
	}
	return n, nil
}

// Incr increments the integer stored at key and returns the new value.
// If the key does not exist it is initialised to 0 before incrementing.
func (c *Client) Incr(key string) (int64, error) {
	resp, err := c.do("INCR", key)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(resp, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("redisclient: Incr: unexpected response: %s", resp)
	}
	return n, nil
}

// ---- low-level send/receive ----

// do executes a command and returns the response as a string.
func (c *Client) do(args ...string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.send(args...); err != nil {
		return "", err
	}
	val, err := c.readResponse()
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", nil
	}
	return *val, nil
}

// doRaw executes a command and returns a nullable string pointer.
func (c *Client) doRaw(args ...string) (*string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.send(args...); err != nil {
		return nil, err
	}
	return c.readResponse()
}

// send writes a RESP array to the connection.
func (c *Client) send(args ...string) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*%d\r\n", len(args)))
	for _, arg := range args {
		sb.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg))
	}
	_, err := fmt.Fprint(c.conn, sb.String())
	return err
}

// readResponse reads a single RESP value from the server.
// Returns nil for null bulk strings, a string pointer otherwise.
func (c *Client) readResponse() (*string, error) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("redisclient: connection closed")
		}
		return nil, err
	}
	line = strings.TrimRight(line, "\r\n")
	if len(line) == 0 {
		return nil, fmt.Errorf("redisclient: empty response line")
	}

	prefix := line[0]
	rest := line[1:]

	switch prefix {
	case '+': // Simple string
		return &rest, nil

	case '-': // Error
		return nil, fmt.Errorf("redisclient: server error: %s", rest)

	case ':': // Integer
		return &rest, nil

	case '$': // Bulk string
		n, err := strconv.Atoi(rest)
		if err != nil {
			return nil, fmt.Errorf("redisclient: bulk string length: %w", err)
		}
		if n == -1 {
			return nil, nil // nil bulk string
		}
		buf := make([]byte, n+2) // +2 for \r\n
		if _, err := io.ReadFull(c.reader, buf); err != nil {
			return nil, err
		}
		s := string(buf[:n])
		return &s, nil

	case '*': // Array (we only need flat string for most responses)
		count, err := strconv.Atoi(rest)
		if err != nil {
			return nil, fmt.Errorf("redisclient: array count: %w", err)
		}
		if count == -1 {
			return nil, nil
		}
		// Read all array elements and join with newline for simplicity.
		parts := make([]string, 0, count)
		for i := 0; i < count; i++ {
			v, err := c.readResponse()
			if err != nil {
				return nil, err
			}
			if v != nil {
				parts = append(parts, *v)
			}
		}
		joined := strings.Join(parts, "\n")
		return &joined, nil

	default:
		return nil, fmt.Errorf("redisclient: unknown RESP prefix: %c", prefix)
	}
}
