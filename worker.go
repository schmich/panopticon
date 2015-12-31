package main

import (
  "net"
  "fmt"
  "log"
  "strings"
  "time"
  "strconv"
  "os"
  "encoding/binary"
)

type TwitchLogger struct {
  conn net.Conn
  listening chan bool
  join chan string
  file *os.File
}

func Connect(hostname string, port int) *TwitchLogger {
  conn, err := net.Dial("tcp", hostname + ":" + strconv.Itoa(port))
  if err != nil {
    log.Fatal("Cannot connect to IRC server: ", err)
  }

  fmt.Printf("Connected to %s:%d.\n", hostname, port)

  file, err := os.Create("twitch-" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".txt")
  if err != nil {
    log.Fatal("Cannot create file: ", err)
  }

  writeTimestamp(file, make([]byte, 8))

  twitch := &TwitchLogger {
    conn: conn,
    listening: make(chan bool, 1),
    join: make(chan string, 100),
    file: file,
  }

  go twitch.read()

  return twitch
}

func writeTimestamp(file *os.File, timestampBuf []byte) {
  timestamp := time.Now().UnixNano()
  binary.LittleEndian.PutUint64(timestampBuf, uint64(timestamp))
  file.Write(timestampBuf)
}

func (twitch *TwitchLogger) read() {
  var timestampBuf [8]byte
  buf := make([]byte, 4096)
  for {
    count, err := twitch.conn.Read(buf)
    if err != nil {
      log.Fatal("Cannot read from IRC server: ", err)
    }

    start := 0
    i := 0
    for ; i < count; i++ {
      if buf[i] == 10 {
        twitch.file.Write(buf[start:i + 1])
        writeTimestamp(twitch.file, timestampBuf[:])
        start = i + 1
      }
    }

    twitch.file.Write(buf[start:i])
  }
}

func (twitch *TwitchLogger) sendCommand(command string) {
  fmt.Fprintf(twitch.conn, "%s\r\n", command)
}

func (twitch *TwitchLogger) joinChannels() {
  for {
    channel := "#" + <-twitch.join
    fmt.Printf("Joining %s\n", channel)
    twitch.sendCommand("JOIN " + channel)
    time.Sleep(1000 * time.Millisecond)
  }
}

func (twitch *TwitchLogger) sendPong() {
  for {
    time.Sleep(10 * time.Second);
    twitch.sendCommand("PONG tmi.twitch.tv")
  }
}

func (twitch *TwitchLogger) Login(username string, password string) {
  username = strings.ToLower(username)
  twitch.sendCommand("USER " + username)
  twitch.sendCommand("NICK " + username)
  twitch.sendCommand("CAP REQ :twitch.tv/tags")
  twitch.sendCommand("CAP REQ :twitch.tv/commands")
  twitch.sendCommand("CAP REQ :twitch.tv/membership")

  go twitch.joinChannels();
  go twitch.sendPong();
}

func (twitch *TwitchLogger) Listen() {
  <-twitch.listening
}

func (twitch *TwitchLogger) Join(channel string) {
  if channel[0] == '#' {
    channel = channel[1:]
  }

  twitch.join <- strings.ToLower(strings.TrimSpace(channel))
}

func main() {
  channels := os.Args[1:]

  clientCount := (len(channels) / 100) + 1
  clients := make([]*TwitchLogger, 0, clientCount)

  for i := 0; i < clientCount; i++ {
    client := Connect("irc.twitch.tv", 6667)
    client.Login("justinfan0", "")
    clients = append(clients, client)
  }

  for i, channel := range channels {
    if strings.TrimSpace(channel) == "" {
      continue
    }

    clientIndex := i % clientCount
    clients[clientIndex].Join(channel)
  }

  for _, client := range clients {
    client.Listen()
  }
}
