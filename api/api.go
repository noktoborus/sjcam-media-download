package api

import (
  "fmt"
  "encoding/json"
)

/* message token. You should receive it before call any */
type TokenType = int
/* Type of message */
type MessageType = int
/* rval */
type RVal = int

/* Known RVals */
const (
  Success RVal = 0
  InvalidArguments = -1
  InvalidToken = -4
  FileNotFound = -25
)

const (
  None MessageType = 0
  /* Set time to camera */
  SetTime = 2
  /* Format message: received if user touch 'Format' in setting menu */
  Format = 4
  /* Information event: start record, stop record, enter menu, leave menu, file donwload complete, ... */
  Info = 7
  /* FW Version, Camera Name, etc... */
  CameraInfo = 11
  /* Get charge level */
  BatteryInfo = 13
  /* Token Get */
  Token = 257
  /* Set receive address before file download */
  PermitReceiver = 261
  /* Download file */
  GetFile = 1285
  /* Another connection acquire a token. Current connection is bad. */
  Closed = 1793
  /* list of available media: images and videos */
  MediaList = 2049
  /* Switch off RTSP flow */
  SetRTSPOff = 2051
  /* Switch on RTSP flow */
  SetRTSPOn = 2052
)

/************************ Requests *************************/
type Request struct {
  MsgType MessageType `json:"msg_id"`
  Token TokenType `json:"token"`
}

type TokenRequest struct {
  MsgType MessageType `json:"msg_id"`
}

func (TokenRequest) New() TokenRequest {
  return TokenRequest{Token} 
}

type BatteryInfoRequest struct {
  MsgType MessageType `json:"msg_id"`
  Token TokenType `json:"token"`
}

func (BatteryInfoRequest) New(token TokenType) BatteryInfoRequest {
  return BatteryInfoRequest{BatteryInfo, token} 
}

type MediaListRequest struct {
  MsgType MessageType `json:"msg_id"`
  Token TokenType `json:"token"`
}

func (MediaListRequest) New(token TokenType) MediaListRequest {
  return MediaListRequest{MediaList, token} 
}

/* This request does not have response */
type SetRTSPOffRequest struct {
  MsgType MessageType `json:"msg_id"`
  Token TokenType `json:"token"`
}

func (SetRTSPOffRequest) New(token TokenType) SetRTSPOffRequest {
  return SetRTSPOffRequest{SetRTSPOff, token} 
}

/* This request does not have response */
type SetRTSPOnRequest struct {
  MsgType MessageType `json:"msg_id"`
  Token TokenType `json:"token"`
}

func (SetRTSPOnRequest) New(token TokenType) SetRTSPOnRequest {
  return SetRTSPOnRequest{SetRTSPOn, token} 
}

type PermitReceiverRequest  struct {
  MsgType MessageType `json:"msg_id"`
  Token TokenType `json:"token"`

  Address string `json:"param"`
  Proto string `json:"type"`
}

func (PermitReceiverRequest) New(token TokenType, address string, proto string) PermitReceiverRequest {
  if proto != "TCP" && proto != "UDP" {
    panic("PermitReceiverRequest: only TCP and UDP allowed to proto");
  }
  
  return PermitReceiverRequest{PermitReceiver, token, address, proto} 
}

type GetFileRequest struct {
  MsgType MessageType `json:"msg_id"`
  Token TokenType `json:"token"`

  Offset uint64 `json:"offset"`
  Path string `json:"param"`
}

func (GetFileRequest) New(token TokenType, offset uint64, path string) GetFileRequest {
  return GetFileRequest{GetFile, token, offset, path} 
}

type CameraInfoRequest struct {
  MsgType MessageType `json:"msg_id"`
  Token TokenType `json:"token"`
}

func (CameraInfoRequest) New(token TokenType) CameraInfoRequest {
  return CameraInfoRequest{CameraInfo, token}
}

/************************ Responses *************************/
type Response struct {
  RVal RVal `json:"rval"`
  MsgType MessageType `json:"msg_id"`
}

type TokenResponse struct {
  RVal RVal `json:"rval"`
  MsgType MessageType `json:"msg_id"`

  Token TokenType `json:"param"`
}

type MediaListResponse struct {
  RVal RVal `json:"rval"`
  MsgType MessageType `json:"msg_id"`

  Index uint `json:"index"`
  Total uint `json:"total"`
  Media []string `json:"param"`
}

type GetFileResponse struct {
  RVal RVal `json:"rval"`
  MsgType MessageType `json:"msg_id"`

  /* != size if Offset in request setted? */
  RemainSize uint64 `json:"rem_size"`
  /* full size of file */
  Size string `json:"size"`
}

type CameraInfoResponse struct {
  RVal RVal `json:"rval"`
  MsgType MessageType `json:"msg_id"`

  Brand string `json:"brand"`
  Model string `json:"model"`
  Chip string `json:"chip"`
  APIVersion string `json:"api_ver"`
  MediaFolder string `json:"media_folder"`
  EventFolder string `json:"event_folder"`
  FirmwareVersion string `json:"firmwareVersion"`
}

type BatteryInfoResponse struct {
  RVal RVal `json:"rval"`
  MsgType MessageType `json:"msg_id"`

  PowerSupply string `json:"type"`
  ChargePercent uint `json:"param"`
}

type PermitReceiverResponse struct {
  RVal RVal `json:"rval"`
  MsgType MessageType `json:"msg_id"`
}

type API struct {
  /* responses */
  DoGetFileResponse func(GetFileResponse)
  DoPermitReceiver func(PermitReceiverResponse)
  DoMediaList func(MediaListResponse)
  DoGetFile func(GetFileResponse)
  DoToken func(TokenResponse)
  DoCameraInfo func(CameraInfoResponse)
  DoBatteryInfo func(BatteryInfoResponse)
  DoError func(Response)
  /* (Required) parsing problem */
  GenericError func(string)
  /* (Required) got unsupported message */
  Unsupported func(json.RawMessage)
  /* (Required) known message w/o handler */
  NoHandler func(string, json.RawMessage)
}

func DoResponse(state *API, rawMessage json.RawMessage) {
  var response Response

  if state.Unsupported == nil {
    panic("'API->Unsupported' callback must be defined")
  }

  if state.GenericError == nil {
    panic("'API->GenericError' callback must be defined")
  }

  if state.NoHandler == nil {
    panic("'API->NoHandler' callback must be defined")
  }

  defer func() {
    if r := recover(); r != nil {
      state.GenericError(fmt.Sprintf("%s", r))
    }
  } ()

  json.Unmarshal(rawMessage, &response)
  if( response.RVal != 0 ) {
    if state.DoError != nil {
      state.DoError(response)
    } else {
      state.NoHandler("DoError", rawMessage)
    }
    return
  }

  switch response.MsgType {
  case Token:
    var response TokenResponse

    if err := json.Unmarshal(rawMessage, &response); err != nil {
      panic(err)
    } else if state.DoToken != nil {
      state.DoToken(response)
    } else {
      state.NoHandler("DoToken", rawMessage)
    }
  case PermitReceiver:
    var response PermitReceiverResponse

    if err := json.Unmarshal(rawMessage, &response); err != nil {
      panic(err)
    } else if state.DoPermitReceiver != nil {
      state.DoPermitReceiver(response)
    } else {
      state.NoHandler("DoPermitReceiver", rawMessage)
    }
  case GetFile:
    var response GetFileResponse

    if err := json.Unmarshal(rawMessage, &response); err != nil {
      panic(err)
    } else if state.DoGetFile != nil {
      state.DoGetFile(response)
    } else {
      state.NoHandler("DoGetFile", rawMessage)
    }
  case MediaList:
    var response MediaListResponse

    if err := json.Unmarshal(rawMessage, &response); err != nil {
      panic(err)
    } else if state.DoMediaList != nil {
      state.DoMediaList(response)
    } else {
      state.NoHandler("DoMediaList", rawMessage)
    }
  case CameraInfo:
    var response CameraInfoResponse

    if err := json.Unmarshal(rawMessage, &response); err != nil {
      panic(err)
    } else if state.DoCameraInfo != nil {
      state.DoCameraInfo(response)
    } else {
      state.NoHandler("DoCameraInfo", rawMessage)
    }
  case BatteryInfo:
    var batteryInfo BatteryInfoResponse

    if err := json.Unmarshal(rawMessage, &batteryInfo); err != nil {
      panic(err)
    } else if state.DoBatteryInfo != nil {
      state.DoBatteryInfo(batteryInfo)
    } else {
      state.NoHandler("DoBatteryInfo", rawMessage)
    }
  default:
    state.Unsupported(rawMessage)
  }
}