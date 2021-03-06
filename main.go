package main

import (
  "time"
  "os"
  "strconv"
  "strings"
  "net"
  "fmt"
  "encoding/json"
  "./api"
)

/* port and address of camera */
const cameraIP = "192.168.42.1"
/* data control port */
const cameraPort = 7878
/* media receive port */
const receivePort = 8787

type ReceiverTask struct {
  filename string
  offset uint64
  size uint64
}

func receiveFile(dataAddress string, task ReceiverTask) {
  var buf = make([]byte, 4096)
  var conn net.Conn
  var err error
  var receivedBytes uint64
  var expectedBytes uint64
  var file *os.File

  fmt.Printf("Connect to data channel: %s\n", dataAddress)
  conn, err = net.Dial("tcp", dataAddress)
  if err != nil {
    fmt.Printf("... connection to %s failed: %s\n", dataAddress, err)
    return
  }
  defer conn.Close()

  fmt.Printf("Truncate %q to %d\n", task.filename, task.offset)
  err = os.Truncate(task.filename, int64(task.offset))
  if err != nil {
    fmt.Printf("... truncate file to %d bytes failed: %s\n", task.offset, err)
  }

  fmt.Printf("Open %q for writing\n", task.filename)
  file, err = os.OpenFile(task.filename, os.O_CREATE | os.O_APPEND | os.O_RDWR, 0666)
  if err != nil {
    fmt.Printf("... file open return an error: %s\n", err)
    return
  }
  defer file.Close()
  _, err = file.Seek(int64(task.offset), os.SEEK_SET)
  if err != nil {
    fmt.Printf("... seek to offset %d failed: %s\n", task.offset, err)
  }

  expectedBytes = task.size - task.offset
  fmt.Printf("Download task for file: %q (full size: %d, reaming: %d)\n",
             task.filename, task.size, expectedBytes)
  timeStart := time.Now()
  speed := 0.0
  for {
    bytes, err := conn.Read(buf)
    if err != nil {
      fmt.Printf("File not received by error: %s\n", err)
      break
    } else {
      receivedBytes += uint64(bytes)
    }

    file.Write(buf[:bytes])

    if receivedBytes >= expectedBytes {
      fmt.Printf("Download complete: %d trail bytes\n", receivedBytes - expectedBytes)
      break
    }

    timeElapsed := time.Since(timeStart)
    currentSpeed := float64(receivedBytes) / 1024 / 1024 / timeElapsed.Seconds()
    if currentSpeed > speed + 0.5 || currentSpeed < speed -1 {
      speed = currentSpeed
      fmt.Printf("current download speed is %.2fMB/s\n", speed)
    }
  }
  timeElapsed := time.Since(timeStart)
  fmt.Printf("Download speed: %.2fMB/s (%.2fMB in %s)\n",
             float64(receivedBytes) / 1024 / 1024 / timeElapsed.Seconds(),
             float64(receivedBytes) / 1024 / 1024,
             timeElapsed)
}

type FileSaveInfo struct {
  FileName string
  Offset uint64
  Size uint64
}

func getLocalPath(filename string) string {
  return "media/" + filename
}

func getRemotePath(mediaFolder string, filename string) string {
  /* I think MediaListResponse.Index must use somehow */
  return mediaFolder + "/100MEDIA/" + filename
}

func setupCallback(outboundIP string, dataAddress string,
                   encoder *json.Encoder, decoder *json.Decoder,
                   endLoop func(),
                   state *api.API) {
  /* API token */
  var token api.TokenType
  /* download file queue */
  var reamingFiles []FileSaveInfo
  /* Media directory on device. Need for file download */
  var mediaFolder string

  var downloadFiles = func() {
    var filesCount = len(reamingFiles)
    var remoteFilePath string

    fmt.Printf("Files to save: %d\n", filesCount)
    if filesCount == 0 {
      fmt.Printf("All files saved. Have a good day!\n")
      endLoop()
      return
    }

    remoteFilePath = getRemotePath(mediaFolder, reamingFiles[0].FileName)
    fmt.Printf("Download file: %s\n", remoteFilePath)
    encoder.Encode((&api.GetFileRequest{}).New(token, reamingFiles[0].Offset, remoteFilePath))
  }

  state.DoError = func(response api.Response) {
    if response.RVal == api.InvalidToken {
      encoder.Encode((&api.TokenRequest{}).New())
      return
    }

    if response.MsgType == api.Token && response.RVal == api.CameraNotReady {
      fmt.Printf("Camera do not allow run commands: may be you view pictures on device's screen")
      endLoop()
      return
    }

    if response.MsgType == api.MediaList && response.RVal == api.InvalidArguments {
      fmt.Printf("Nothing to save: media storage is empty\n")
      endLoop()
      return
    }

    if response.MsgType == api.GetFile && response.RVal == api.FileNotFound {
      fmt.Printf("Device report what file not exists. " +
                 "I think need use 'GetFile.Index' for get '/<N>MEDIA/' directory.")
      endLoop()
      return
    }

    fmt.Printf("Unknown Error: MsgType=%d RVal=%d\n", response.MsgType, response.RVal)
  }

  state.DoToken = func(tokenResponse api.TokenResponse) {
    token = tokenResponse.Token

    encoder.Encode((api.CameraInfoRequest{}).New(tokenResponse.Token))
  }

  state.DoCameraInfo = func(cameraInfo api.CameraInfoResponse) {
    fmt.Printf("You device is %s %s (Chip: %s) FW %s API %s\n",
               cameraInfo.Brand, cameraInfo.Model, cameraInfo.Chip,
               cameraInfo.FirmwareVersion, cameraInfo.APIVersion)
    fmt.Printf("Media Folder: %s\n", cameraInfo.MediaFolder)
    fmt.Printf("Event Folder: %s\n", cameraInfo.EventFolder)
    encoder.Encode((api.MediaListRequest{}).New(token))
    mediaFolder = cameraInfo.MediaFolder
  }

  state.DoMediaList = func(mediaList api.MediaListResponse) {
    fmt.Printf("Media on device:\n")
    for index, media := range mediaList.Media {
      var info = strings.Split(media, ",")
      var localFile = getLocalPath(info[0])
      var offset uint64 = 0
      size, _ := strconv.ParseUint(info[len(info)-1], 10, 64)

      fmt.Printf("%03d. %s\n", index + 1, media)
      /* check local file */
      if info, err := os.Stat(localFile); err == nil {
        if( info.Size() == int64(size) ) {
          fmt.Printf("   - skip: already downloaded\n")
          continue
        } else if ( info.Size() > int64(size) ) { 
          fmt.Printf("   - skip: local size %d > remote %d\n", info.Size(), size)
        } else if( info.Size() < int64(size) ) {
          offset = uint64(info.Size())
          fmt.Printf("   - queued %d/%d bytes for download\n", size - offset, size)
        } else {
          fmt.Printf("   - queued for download\n")
        }
      } else {
        fmt.Printf("   - queued for download\n")
      }
      /* append file to download queue */
      reamingFiles = append(reamingFiles, FileSaveInfo{info[0], offset, size})
    }
    fmt.Printf("[Index %d Total %d]\n", mediaList.Index, mediaList.Total)
    /* SJCAM API do not allow send command chunks e.g. ({msg_id:1}{msg_id:2})
     * need wait time to send next command or send after camera event
     * I think it better chanin for download:
     * 1. MediaList
     * 2. BatteryInfo
     * 3. PermitReceiver
     * 4. GetFile
     * 5. go to step 2 if download queue not empty
     */
    encoder.Encode((api.BatteryInfoRequest{}).New(token))
  }

  state.DoBatteryInfo = func(batterInfo api.BatteryInfoResponse) {
    fmt.Printf("Power: %d%% (source: %s)\n", batterInfo.ChargePercent, batterInfo.PowerSupply)
    encoder.Encode((&api.PermitReceiverRequest{}).New(token, outboundIP, "TCP"))
  }

  state.DoPermitReceiver = func(response api.PermitReceiverResponse) {
    fmt.Printf("Address %s set as reserved successfully. Now try download file.\n", outboundIP)
    downloadFiles()
  }

  state.DoGetFile = func(getFile api.GetFileResponse) {
    var localFileName = getLocalPath(reamingFiles[0].FileName)

    size, _ := strconv.ParseUint(getFile.Size, 10, 64)
    offset := uint64(size) - getFile.RemainSize
    receiveFile(dataAddress,
                ReceiverTask{localFileName, offset, uint64(size)})
    fmt.Printf("Remove %q from download queue\n", reamingFiles[0].FileName)
    reamingFiles = reamingFiles[1:len(reamingFiles)]
    encoder.Encode((api.BatteryInfoRequest{}).New(token))
  }

  state.GenericError = func(err string) {
    panic(err)
  }

  state.NoHandler = func(name string, rawMessage json.RawMessage) {
    fmt.Printf("NoHandled(%s): %q\n", name, rawMessage)
  }

  state.Unsupported = func(unsupportedMessage json.RawMessage) {
    fmt.Printf("Unsupported: %q\n", unsupportedMessage)
  }
}

func runControl(outboundIP string, dataAddress string,
                encoder *json.Encoder, decoder *json.Decoder) {
  var state api.API
  var isRunning bool = true

  encoder.Encode((&api.TokenRequest{}).New())

  setupCallback(outboundIP, dataAddress,
                encoder, decoder,
                func() { isRunning = false },
                &state)
  for isRunning {
    var message json.RawMessage

    err := decoder.Decode(&message)
    if err != nil {
      fmt.Printf("Communication with device failed: %s\n", err)
      return
    }

    api.DoResponse(&state, message)
  }
}

func main() {
  var address = fmt.Sprintf("%s:%d", cameraIP, cameraPort)
  var dataAddress = fmt.Sprintf("%s:%d", cameraIP, receivePort)
  var localAddress string

  fmt.Printf("Hello.\n")
  fmt.Printf("Now I connect to %s and try to receive all media from %s.\n", address, dataAddress)

  conn, err := net.Dial("tcp", address)
  if err != nil {
    fmt.Printf("Cannot connect to %s: %s\n", address, err)
    return
  }
  defer conn.Close()
  localAddress = fmt.Sprintf("%s", conn.LocalAddr())

  fmt.Printf("Address %s used as source for download\n", localAddress);

  decoder := json.NewDecoder(conn)
  encoder := json.NewEncoder(conn)

  runControl(localAddress, dataAddress, encoder, decoder)
}