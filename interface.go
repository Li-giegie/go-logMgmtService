package main

type LogMgmtServiceI interface {
	Serve()
	FindLogFile(name string) ([]FindLogFile, error)
	FindLogInfo(key string) ([]string,error)
	DelLogFile(key string) (string,error)
}
