# go-logMgmtService 是一个通过配置文件实现管理日志的服务

- [支持对指定目录内所有或自定义标识的文件 进行管理](#)
- [支持对指定路径的日志文件进行管理](#)

![go1.19](https://img.shields.io/badge/Go-1.19-blue)
![log](https://img.shields.io/badge/Log-Mgmt-yellow)

## 目录结构
    go-logMgmtService/
        conf.yaml           //配置文件
        go.mod              //go.mod
        main.go             //主入口 功能实现
        readme.md           //readme
        
## 配置文件

#### 配置项目分为[公共配置项](#)和[私有配置项](#)：当私有配置为缺省或为对应类型的初始值或空值时，将采用公共配置
```yaml
app:
- name:               # 应用名 （体现在打包后的压缩文件名中）
  log_dir:            # 日志目录或日志路径，可配置多项
    - ./
    - ./test/test.log
  out_of_data: 3      # 私有配置 过期压缩时间 单位秒
  log_tag:            # 私有配置 当 log_dir 配置项为目录且目录内有不受本程序管理的日志文件时，可选用此项作为过滤包含指定特征的日志文件 (当配置多项时关系为 ”或“ 满足任一条件视为非过滤项 ) 为 ""空字符或 "*" 时表示不进行过滤。 
    - ".log"
  save_path: "../ip"  # 私有配置 本应用日志打包文件 ["" 使用公共配置项 | "%pub%" 使用公共配置项 |"%auto%" 保存到日志同级目录]
pack_algorithm: zip   # 打包格式 仅支持ZIP
out_of_data: 86400    # 公共配置 过期时间
log_tag:              # 公共属性 同上
  - ".log"
save_path: ""         # 公共属性 同上
```
### 数据结构
```go
    type LogMgmtService struct {
        App []_appService	`yaml:"app"`
        // 打包算法 仅支持zip
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
```

### 启动命令

[命令参数包含特殊字符使用 单双引号包括（'单引号参数'、"双引号参数"）](#)

* -conf "./conf.yaml" # 指定配置文件运行 默认值 ./conf.yaml
    
* -createconf ""      # 创建配置文件模板 参数文件名 例："./conf1.yaml"
