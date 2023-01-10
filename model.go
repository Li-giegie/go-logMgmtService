package main

const FindLogFile_KEY = "0379@All==fZxPo>"

type FindLogFile struct {
	FileName string `json:"file_name,omitempty"`
	Key      string `json:"key,omitempty"`
	Type     string `json:"type,omitempty"`
}

type _key16 [16]byte
type _key32 [32]byte

type _keyI interface {
	Marshal() []byte
}