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
  "net/http"
  "io/ioutil"
  "encoding/json"
  "net/url"
)

type Servers struct {
  Cluster string `json:"cluster"`
  Servers []string `json:"servers"`
  WebsocketsServers []string `json:"websockets_servers"`
}

func chatServer(channel string) (string, []string, error) {
  resp, err := http.Get("http://tmi.twitch.tv/servers?channel=" + url.QueryEscape(channel))
  if err != nil {
    return "", []string{}, err
  }

  defer resp.Body.Close()
  body, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    return "", []string{}, err
  }

  var response Servers
  err = json.Unmarshal(body, &response)
  if err != nil {
    return "", []string{}, err
  }

  return response.Cluster, response.Servers, nil
}

type TwitchLogger struct {
  conn net.Conn
  listening chan bool
  join chan string
  file *os.File
}

func Connect(endpoint string) *TwitchLogger {
  conn, err := net.Dial("tcp", endpoint)
  if err != nil {
    log.Fatal("Cannot connect to IRC server: ", err)
  }

  fmt.Printf("Connected to %s.\n", endpoint)

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

func (twitch *TwitchLogger) sendPongs() {
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
  go twitch.sendPongs();
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

  clientCount := len(channels)
  clients := make([]*TwitchLogger, 0, clientCount)

  for _, channel := range channels {
    if strings.TrimSpace(channel) == "" {
      continue
    }

    cluster, servers, _ := chatServer(channel)
    server := servers[0]
    fmt.Printf("#%s is hosted on %s (%s).\n", channel, server, cluster)

    client := Connect(server)
    client.Login("justinfan0", "")
    client.Join(channel)
    clients = append(clients, client)
  }

  for _, client := range clients {
    client.Listen()
  }
}
