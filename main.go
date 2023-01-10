package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	_file "github.com/dablelv/go-huge-util/file"
	"github.com/dablelv/go-huge-util/zip"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"path"
	"strconv"
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

	// 打包的zip包的命名方式 [日期-时间] [分隔符] [应用名][扩展名]  分隔符 默认值 "@"
	// NameSplitStr="@"
	// 示例：2023-01-10 14-16-40@应用1.zip
	NameSplitStr string
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

	// 参考公共字段LogMgmtService解释
	NameSplitStr string
	// 打包保存位置 可选值Auto
	SavePath string	`yaml:"save_path"`

}

var confP = flag.String("conf","./conf.yaml","指定配置文件启动")

func main()  {

	createconf := flag.String("createconf","","生成默认配置文件")

	flag.Parse()

	if *createconf != ""{
		CreateConfFile(*createconf)
	}

	lms,err := NewLogMgmtService(*confP)

	if err != nil {
		log.Fatalln(err)
	}

	lms.Serve()

}

func NewLogMgmtService(confPath ...string) (LogMgmtServiceI,error)   {

	if confPath == nil { confPath = []string{"./conf.yaml"} }

	buf,err := os.ReadFile(confPath[0])
	if err != nil {
		return nil, appendError(errors.New("读取配置文件失败 运行结束："),err)
	}

	var lms LogMgmtService

	err = yaml.Unmarshal(buf,&lms)
	if err != nil {
		return nil, appendError(errors.New("序列化配置文件错误 运行结束："),err)
	}

	var changeInfo = new(bytes.Buffer)

	for index, _ := range lms.App {

		fmt.Println("app：",lms.App[index].Name)
		if lms.App[index].OutOfData <= 0 {
			lms.App[index].OutOfData = lms.OutOfData
			changeInfo.WriteString("[超时时间]未配置或配置不合法启用公共参数："+strconv.Itoa(int(lms.App[index].OutOfData)))
		}
		if lms.App[index].LogTag == nil || len(lms.App[index].LogTag) == 0 {
			lms.App[index].LogTag = lms.LogTag
			changeInfo.WriteString("[日志标记]未配置或配置不合法启用公共参数："+ strings.Join(lms.LogTag," | "))
		}
		if lms.App[index].SavePath == "" {
			if lms.SavePath == ""{
				lms.SavePath = "auto"
				changeInfo.WriteString("[保存路径]未配置或配置不合法启用公共参数：auto")
			}
			lms.App[index].SavePath = lms.SavePath
			if lms.App[index].SavePath == "auto" {
				base,_ := path.Split(lms.App[index].LogDir[0])
				lms.App[index].SavePath = base
			}
		}
		if is,err := _file.IsPathExist(lms.App[index].SavePath); err != nil || !is {
			err = os.MkdirAll(lms.App[index].SavePath,0666)
			if err != nil {
				panic(any("日志输出目录不存在尝试创建失败------"))
			}
		}
		if lms.App[index].SavePath[len(lms.App[index].SavePath)-1:] != "/" && lms.App[index].SavePath[len(lms.App[index].SavePath)-1:] != `\` {
			lms.App[index].SavePath += "/"
			changeInfo.WriteString("[保存路径]目录自动补全/")
		}

		if lms.App[index].NameSplitStr == "" {
			if lms.NameSplitStr == "" {
				lms.NameSplitStr = "@"
			}
			lms.App[index].NameSplitStr = lms.NameSplitStr
			changeInfo.WriteString("[保存路径分隔符]未配置或配置不合法启用公共参数：@")
		}

	}
	if changeInfo.String() != ""{
		nbuf,nerr := yaml.Marshal(lms)
		if nerr != nil {
			log.Println("配置文件变更同步至文件-------失败 序列化失败",nerr)
		}else {
			if nerr = os.WriteFile(*confP,nbuf,0666); nerr != nil {
				log.Println("配置文件变更同步至文件-------失败 写入文件",nerr)
			}
		}
	}

	fmt.Println(changeInfo.String())

	return &lms,nil
}

// 开启服务
func (l *LogMgmtService) Serve()  {

	for _, app := range l.App {
		go func(appService _appService) {
			appService.Execute()
		}(app)

	}

	select {}
	
}

// 查找日志文件列表
// 入参app name 通常为服务名 返回值 为文件名和key
func (l *LogMgmtService) FindLogFile(name string) ([]FindLogFile,error) {
	var findLogFile = make([]FindLogFile,0)
	for _, service := range l.App {
		if service.Name == name {
			zipPack,err := getFiles([]string{service.SavePath},[]string{service.Name},false)
			if err != nil{
				return findLogFile,appendError("获取压缩包目录失败：",err)
			}
			logFile,err := service.getFiles()
			if err != nil{
				return findLogFile,appendError("获取日志文件目录失败",err)
			}
			for _, lf := range logFile {
				findLogFile = append(findLogFile, FindLogFile{
					FileName: lf,
					Key:      AesEncrypt(lf,NewKey(FindLogFile_KEY)),
					Type:     "file",
				})
			}
			for _, zp := range zipPack {
				findLogFile = append(findLogFile, FindLogFile{
					FileName: zp,
					Key:      AesEncrypt(zp,NewKey(FindLogFile_KEY)),
					Type:     "zip",
				})
			}

			break
		}
	}

	return findLogFile,nil
}

// 查找文件内容返回路径
func (l *LogMgmtService) FindLogInfo(key string) ([]string,error) {
	_path := AesDecrypt(key,NewKey(FindLogFile_KEY))
	if ok,err := _file.IsPathExist(_path); err != nil || !ok {
		return nil,appendError("key不存在非法Key：",err)
	}
	exit := path.Ext(_path)
	switch exit {
	case ".log":
		return []string{_path},nil
	case ".zip":
		return []string{_path},nil
	default:
		log.Println("waring main.FindLogInfo ：默认支持查找的的文件类型扩展名是 [.log 或 .zip] 不支持：",exit)
		return nil,appendError("默认支持查找的的文件类型扩展名是 [.log 或 .zip] 不支持：",exit)
	}
}

// 删除日志文件内容返回路径
func (l *LogMgmtService) DelLogFile(key string) (string,error) {
	return key,os.Remove(AesDecrypt(key,NewKey(FindLogFile_KEY)))
}

func (a *_appService) Execute()  {

	setInterval(time.Duration(a.OutOfData), func() {

		files,err := a.getFiles()
		if err != nil {
			log.Fatalln(err)
		}

		_,isRefresh := a.filter(files)

		if !isRefresh {
			return
		}

		fileName := time.Now().Format("2006-01-02 15-04-05") +"@" + a.Name +".zip"
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

	return getFiles(a.LogDir,a.LogTag)
	//var resultFile []string
	//for _, dir := range a.LogDir {
	//
	//	finfo,err := os.Stat(dir)
	//	if err != nil {
	//		return nil,appendError(errors.New("打开路径失败 -1："),err)
	//	}
	//
	//	filePaths := []string{dir}
	//
	//	if finfo.IsDir() {
	//		if filePaths,err = _file.GetDirAllEntryPaths(dir,true); err != nil {
	//			return nil,appendError(errors.New("打开路径失败 -2："),err)
	//		}
	//	}
	//	for _, filePath := range filePaths {
	//		for _, s2 := range a.LogTag {
	//			if strings.Contains(filePath,s2) || s2 == "" || s2 == "*" {
	//				resultFile = append(resultFile, filePath)
	//				break
	//			}
	//		}
	//	}
	//}
	//
	//return resultFile,nil
}

func (a _appService) filter(files []string) ([]string,bool) {

	fmt.Println("filter before：",a.Name,files)

	var _files = make([]string,0)
	var refresh bool
	for _, s := range files {

		info,err := os.Stat(s)
		if err != nil {
			log.Println("_appService filter error:",err)
			continue
		}
		if info.Size() == 0 {
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
	fmt.Printf("filter later：Name %v refresh %v %v\n",a.Name,refresh,_files)
	return _files,refresh
}

// rules：规则 当检索的路径不包含指定字符串（"" , "*" 表示不过滤）后视后视为过滤掉的路径，否则反之
func getFiles(_paths []string,rules []string,reserveDir ...bool) ([]string,error)  {
	if reserveDir == nil { reserveDir = []bool{true} }
	var resultFile []string
	var isAdd bool
	for _, dir := range _paths {

		finfo,err := os.Stat(dir)
		if err != nil {
			return nil,appendError(errors.New("打开路径失败 -1："),err)
		}

		filePaths := []string{dir}

		if finfo.IsDir() {
			if filePaths,err = _file.GetDirAllEntryPaths(dir,reserveDir[0]); err != nil {
				return nil,appendError(errors.New("打开路径失败 -2："),err)
			}
		}
		isAdd = true
		for _, filePath := range filePaths {
			filePath = strings.ReplaceAll(strings.ReplaceAll(filePath,`\`,"/"),"//","/")
			for _, s := range resultFile {
				if s == filePath {
					isAdd = false
					break
				}
			}
			if !isAdd { continue }
			for _, s2 := range rules {
				if strings.Contains(filePath,s2) || s2 == "" || s2 == "*" {
					resultFile = append(resultFile, filePath)
					break
				}
			}
		}
	}

	return resultFile,nil
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

func appendError (err ...interface{}) error {

	var newStr = new(bytes.Buffer)

	for _, err2 := range err {
		newStr.WriteString(fmt.Sprintf("%v",err2))
	}

	return errors.New(newStr.String())
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

func AesEncrypt(orig string, key _keyI) string {
	// 转成字节数组
	origData := []byte(orig)
	k := key.Marshal()

	// 分组秘钥
	block, err := aes.NewCipher(k)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	// 获取秘钥块的长度
	blockSize := block.BlockSize()
	// 补全码
	origData = PKCS7Padding(origData, blockSize)
	// 加密模式
	blockMode := cipher.NewCBCEncrypter(block, k[:blockSize])
	// 创建数组
	cryted := make([]byte, len(origData))
	// 加密
	blockMode.CryptBlocks(cryted, origData)

	return base64.StdEncoding.EncodeToString(cryted)

}

func AesDecrypt(cryted string, key _keyI) string {
	// 转成字节数组
	crytedByte, _ := base64.StdEncoding.DecodeString(cryted)
	k := key.Marshal()

	// 分组秘钥
	block, _ := aes.NewCipher(k)
	// 获取秘钥块的长度
	blockSize := block.BlockSize()
	// 加密模式
	blockMode := cipher.NewCBCDecrypter(block, k[:blockSize])
	// 创建数组
	orig := make([]byte, len(crytedByte))
	// 解密
	blockMode.CryptBlocks(orig, crytedByte)
	// 去补全码
	orig = PKCS7UnPadding(orig)
	return string(orig)
}

//补码
func PKCS7Padding(ciphertext []byte, blocksize int) []byte {
	padding := blocksize - len(ciphertext)%blocksize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

//去码
func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func NewKey(key string) _keyI {
	tmp := []byte(key)

	if len(tmp) <= 16 {
		var tmpK _key16
		for i, b := range tmp {
			tmpK[i] = b
		}
		return tmpK
	}

	var tmpK _key32
	for i, b := range tmp {
		if i >= 32 { break }
		tmpK[i] = b
	}

	return tmpK

}

func (k _key16) Marshal() []byte {
	return k[:16]
}

func (k _key32) Marshal() []byte {
	return k[:32]
}