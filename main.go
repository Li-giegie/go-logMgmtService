package main

import (
	"errors"
	"flag"
	"fmt"
	_file "github.com/dablelv/go-huge-util/file"
	"github.com/dablelv/go-huge-util/zip"

	"gopkg.in/yaml.v3"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

var cacheInfo sync.Map

type LogMgmtService struct {
	App []_appService	`yaml:"app"`

	// 打包算法 zip
	PackAlgorithm string	`yaml:"pack_algorithm"`

	// 公共过期时间 单位秒
	OutOfData int64		`yaml:"out_of_data"`

	// 公共日志文件标识
	LogTag []string	`yaml:"log_tag"`

	// 公共打包保存位置 可选值Auto
	SavePath string	`yaml:"save_path"`
}

type _appService struct {
	// 服务名
	Name string	`yaml:"name"`

	// 日志目录
	LogDir []string	`yaml:"log_dir"`

	// 过期时间 单位秒
	OutOfData int64	`yaml:"out_of_data"`

	// 日志文件标识
	LogTag []string	`yaml:"log_tag"`

	// 打包保存位置 可选值Auto
	SavePath string	`yaml:"save_path"`

}

func main()  {

	confP := flag.String("conf","./conf.yaml","指定配置文件启动")

	createconf := flag.String("createconf","","生成默认配置文件")
	flag.Parse()

	if *createconf != ""{
		CreateConfFile(*createconf)
	}

	lms,err := NewLogMgmtService(*confP)

	if err != nil {
		log.Fatalln(err)
	}

	lms.TimerEvent()

}


func NewLogMgmtService(path ...string) (*LogMgmtService,error)   {

	if path == nil { path = []string{"./conf.yaml"} }

	buf,err := os.ReadFile(path[0])
	if err != nil {
		return nil, appendError(errors.New("读取配置文件失败 运行结束："),err)
	}

	var lms LogMgmtService

	err = yaml.Unmarshal(buf,&lms)
	if err != nil {
		return nil, appendError(errors.New("序列化配置文件错误 运行结束："),err)
	}

	return &lms,nil
}

func (l *LogMgmtService) TimerEvent()  {

	for _, service := range l.App {

		fmt.Println("app：",service.Name)
		if service.OutOfData <= 0 {
			service.OutOfData = l.OutOfData
		}
		if service.LogTag == nil || len(service.LogTag) == 0 {
			service.LogTag = l.LogTag
		}
		if service.SavePath == "" {
			if l.SavePath == ""{
				l.SavePath = "auto"
			}
			service.SavePath = l.SavePath
			if service.SavePath == "auto" {
				base,_ := path.Split(service.LogDir[0])
				service.SavePath = base
			}
		}

		if service.SavePath[len(service.SavePath)-1:] != "/" && service.SavePath[len(service.SavePath)-1:] != `\` {
			service.SavePath += "/"
		}

		go func(appService _appService) {
			appService.Execute()
		}(service)

	}

	select {}
	
}

func (a *_appService) Execute()  {

	setInterval(time.Duration(a.OutOfData), func() {

		files,err := a.getFiles()
		if err != nil {
			log.Fatalln(err)
		}

		_,isRefresh := a.filter(files)
		fmt.Println("isRefresh ",isRefresh)
		if !isRefresh {
			return
		}

		fileName := time.Now().Format("2006-01-02 15-04-05" +"@" + a.Name +".zip")
		err = zip.Zip(a.SavePath + fileName,files...)
		if err != nil {
			log.Fatalln(err)
		}

		for _, file := range files {
			f,err := os.Create(file)

			if err != nil {
				f.Close()
				log.Println("重置日志错误：",err)
				continue
			}
			info,err := f.Stat()
			if err != nil {
				f.Close()
				log.Println(err)
				continue
			}
			setCache(file,info.ModTime().UnixNano())
			f.Close()
		}
	})
}

func (a *_appService) getFiles() ([]string,error) {

	var resultFile []string
	for _, dir := range a.LogDir {

		finfo,err := os.Stat(dir)
		if err != nil {
			return nil,appendError(errors.New("打开路径失败 -1："),err)
		}

		filePaths := []string{dir}

		if finfo.IsDir() {
			if filePaths,err = _file.GetDirAllEntryPaths(dir,true); err != nil {
				return nil,appendError(errors.New("打开路径失败 -2："),err)
			}
		}
		for _, filePath := range filePaths {
			for _, s2 := range a.LogTag {
				if strings.Contains(filePath,s2) || s2 == "" || s2 == "*" {
					resultFile = append(resultFile, filePath)
					break
				}
			}
		}
	}

	return resultFile,nil
}

func (a _appService) filter(files []string) ([]string,bool) {
	var _files = make([]string,0)
	var refresh bool
	for _, s := range files {

		info,err := os.Stat(s)
		if err != nil {
			log.Println("_appService filter error:",err)
			continue
		}
		v,ok := getCache(s)
		if !ok {
			setCache(s,info.ModTime().UnixNano())
			_files = append(_files, s)
			refresh = true
			continue
		}

		if info.ModTime().UnixNano() != v {
			setCache(s,info.ModTime().UnixNano())
			_files = append(_files, s)
			refresh = true
		}

	}
	fmt.Println("filter ",_files)
	return _files,refresh
}

func setInterval( duration time.Duration,handler func(),close ...chan bool){
	if close == nil { close = make([]chan bool,1)  }
	var t = time.NewTicker(time.Second * duration)
	for  {
		select {
		case <- t.C:
			handler()
		case <-close[0]:
			log.Println("定时器结束任务-------")
			return
		}
	}
}

func setCache(k string,v int64)  {
	cacheInfo.Store(k,v)
}

func getCache(k string) (int64,bool) {
	v,ok := cacheInfo.Load(k)
	if !ok {
		return 0,ok
	}
	return v.(int64),ok
}

func appendError (err ...error) error {

	var newStr string
	for _, err2 := range err {
		newStr += err2.Error()
	}

	return errors.New(newStr)
}

func CreateConfFile(path ...string)  {
	if path == nil { path = []string{"./conf.yaml"} }

	var lgs = LogMgmtService{
		App: []_appService{
			{
				Name:      "检测的应用名",
				LogDir:    []string{"./日志目录"},
				OutOfData: 60*60*24,
				LogTag: []string{".log"},
			},
		},
		SavePath: "打包后日志保存的位置",
		PackAlgorithm: "zip",
		OutOfData: 60*60*24,
		LogTag: []string{".log"},
	}

	buf,err := yaml.Marshal(lgs)

	if err != nil {
		log.Fatalln(err)
	}

	err = os.WriteFile(path[0],buf,0666)

	if err != nil {
		log.Fatalln(err)
	}

	os.Exit(0)
}