package redisclient

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PubSubMessage is a message received from a Redis pub/sub channel.
type PubSubMessage struct {
	Channel string
	Payload string
}

// Subscriber holds a dedicated TCP connection for Redis pub/sub.
// A subscribed connection cannot issue regular GET/SET commands.
// Create a separate Client for normal operations.
type Subscriber struct {
	conn   net.Conn
	reader *bufio.Reader
	msgCh  chan PubSubMessage
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewSubscriber dials a new dedicated connection for pub/sub.
func NewSubscriber(addr string) (*Subscriber, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("redisclient: subscriber dial %s: %w", addr, err)
	}
	return &Subscriber{
		conn:   conn,
		reader: bufio.NewReader(conn),
		msgCh:  make(chan PubSubMessage, 256),
		stopCh: make(chan struct{}),
	}, nil
}

// Subscribe sends the SUBSCRIBE command for the given channels and starts
// the background read loop. Call Messages() to receive incoming messages.
func (s *Subscriber) Subscribe(channels ...string) error {
	if len(channels) == 0 {
		return nil
	}
	args := append([]string{"SUBSCRIBE"}, channels...)
	if err := s.send(args...); err != nil {
		return err
	}
	// Consume the subscribe ack for each channel.
	for range channels {
		if _, err := s.readArray(); err != nil {
			return fmt.Errorf("redisclient: subscribe ack: %w", err)
		}
	}
	// Start the background loop to deliver pushed messages.
	s.wg.Add(1)
	go s.readLoop()
	return nil
}

// Messages returns the channel on which incoming pub/sub messages are delivered.
// The channel is closed when Close is called.
func (s *Subscriber) Messages() <-chan PubSubMessage {
	return s.msgCh
}

// Close stops the subscriber and closes the connection.
func (s *Subscriber) Close() error {
	close(s.stopCh)
	err := s.conn.Close()
	s.wg.Wait()
	return err
}

// readLoop runs in a goroutine, reading server-pushed messages and forwarding
// them to msgCh until the connection is closed or Close is called.
func (s *Subscriber) readLoop() {
	defer s.wg.Done()
	defer close(s.msgCh)

	for {
		elems, err := s.readArray()
		if err != nil {
			// Normal on Close — the connection is intentionally closed.
			return
		}
		if len(elems) == 3 && elems[0] == "message" {
			select {
			case s.msgCh <- PubSubMessage{Channel: elems[1], Payload: elems[2]}:
			case <-s.stopCh:
				return
			}
		}
	}
}

// send writes a RESP array command to the connection.
func (s *Subscriber) send(args ...string) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*%d\r\n", len(args)))
	for _, arg := range args {
		sb.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg))
	}
	_, err := fmt.Fprint(s.conn, sb.String())
	return err
}

// readArray reads a RESP array and returns its elements as strings.
// Handles bulk strings and integers within the array.
func (s *Subscriber) readArray() ([]string, error) {
	line, err := s.reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimRight(line, "\r\n")
	if len(line) == 0 || line[0] != '*' {
		return nil, fmt.Errorf("subscriber: expected array header, got %q", line)
	}
	count, err := strconv.Atoi(line[1:])
	if err != nil {
		return nil, fmt.Errorf("subscriber: array count: %w", err)
	}
	elems := make([]string, 0, count)
	for i := 0; i < count; i++ {
		v, err := s.readValue()
		if err != nil {
			return nil, err
		}
		elems = append(elems, v)
	}
	return elems, nil
}

// readValue reads a single RESP value (bulk string, simple string, or integer).
func (s *Subscriber) readValue() (string, error) {
	line, err := s.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimRight(line, "\r\n")
	if len(line) == 0 {
		return "", fmt.Errorf("subscriber: empty response line")
	}
	switch line[0] {
	case '$':
		n, err := strconv.Atoi(line[1:])
		if err != nil {
			return "", fmt.Errorf("subscriber: bulk string length: %w", err)
		}
		buf := make([]byte, n+2) // +2 for \r\n
		if _, err := io.ReadFull(s.reader, buf); err != nil {
			return "", err
		}
		return string(buf[:n]), nil
	case ':':
		return line[1:], nil
	case '+':
		return line[1:], nil
	default:
		return line[1:], nil
	}
}
