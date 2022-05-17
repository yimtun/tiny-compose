package main

/*
docker inspect  xxx | jq .[].HostConfig.ExtraHosts
*/

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)





var certPath string //  docker 连接时自动判断是否使用证书
var dockerComposeFileName string
var dockerCompsoeFileDir string

func main() {
	certpath, file := cmd()
	certPath = certpath
	dockerComposeFileName = path.Base(file) // 获取docker-compose 文件名字

	fmt.Println("base", path.Base(dockerComposeFileName))

	fmt.Println("dir", path.Dir(dockerComposeFileName)) // 获取文件 所在dir

	abs, err := filepath.Abs(path.Dir(file)) // 获取dir 绝对路径
	if err != nil {

	} else {
		fmt.Println("abs", abs)
		dockerCompsoeFileDir = abs
	}

	fmt.Println("dockerComposeFileName", dockerComposeFileName)
	fmt.Println("dockerCompsoeFileDir", dockerCompsoeFileDir)

	//
	GetConfigFromYml()
	outConfig() //

}

type tmpInterface interface {
	containerConfigFactory()
}

func (a *tmpStruct) containerConfigFactory() {

	fmt.Println(a.containerSpecialPort)

	if len(a.containerDeployTarget) == 0 { // 如果   targetSlice 长度为0 意味着是一个普通的 docker-compsoe 配置

		ymlconfig := ymlConfig{
			volumeSlice:      a.containerVolumeSlice,
			name:             a.containerName,
			environmentSlice: a.templateEnvSlice,
			portMap:          a.containerPortMap,
			volumeMap:        a.containerVolumeMap,

			image:       a.containerImage,
			cmd:         a.containerCmdSlice,
			networkMode: a.containerNetworkMode,
			deployHost:  "localhost",
			capAdd:      a.capacityAdd,
			privileged:  a.containerPrivileged,
			logConfig: a.logConfig,
			extraHosts: a.extraHosts,
			tmpFs: a.tmpFs,
		}
		ymlConfigSlice = append(ymlConfigSlice, ymlconfig)
	} else {
		for _, target := range a.containerDeployTarget {
			ip := (strings.Split(target, ":"))[0]
			port := (strings.Split(target, ":"))[1]
			if port == "0" {
				envs := modifyEnv(a.templateEnvSlice, ip, a.containerSpecialPort) // 如果port 等于0  意味着 在一个机器上只跑一个实例，代表不用修改端口相关配置，这个port应该取原始端口配置第一个容器外端口
				ymlconfig := ymlConfig{
					volumeSlice:      a.containerVolumeSlice,
					name:             a.containerName,
					environmentSlice: envs,
					portMap:          a.containerPortMap,
					volumeMap:        a.containerVolumeMap,

					image:       a.containerImage,
					cmd:         a.containerCmdSlice,
					networkMode: a.containerNetworkMode,
					deployHost:  ip,
					capAdd:      a.capacityAdd,
					privileged:  a.containerPrivileged,
					logConfig: a.logConfig,
					extraHosts: a.extraHosts,
					tmpFs: a.tmpFs,


				}
				ymlConfigSlice = append(ymlConfigSlice, ymlconfig)
			} else {
				// 部署形式为单机多实例    判断手段  port 不为0  port 不为零 说明要修改端口号，端口配置只能有一行 且不能是端口段
				//fmt.Println("单机多实例",name)
				var inPort string
				for _, in := range a.containerPortMap {
					inPort = in
				}
				PortMap := make(map[string]string)
				PortMap[port] = inPort
				// 处理 单机多实例下的端口 ############
				// 处理单机多实例下的 volume
				svcVolumeMap := volumeSlice2Map(a.containerVolumeSlice, port)
				//fmt.Println(svcVolumeMap)
				// 处理单机多实例下的 envs
				envsTem := a.templateEnvSlice
				envs := modifyEnv(envsTem, ip, port)
				ymlconfig := ymlConfig{
					volumeSlice: modifyVolumeSlice(a.containerVolumeSlice, port),
					//volumeSlice: volumeSlice,
					name:             a.containerName + port,
					environmentSlice: envs,
					portMap:          PortMap,
					volumeMap:        svcVolumeMap,

					image:       a.containerImage,
					cmd:         a.containerCmdSlice,
					networkMode: a.containerNetworkMode,
					deployHost:  ip,
					capAdd:      a.capacityAdd,
					privileged:  a.containerPrivileged,
					logConfig: a.logConfig,
					extraHosts: a.extraHosts,
					tmpFs: a.tmpFs,


				}
				ymlConfigSlice = append(ymlConfigSlice, ymlconfig)
			}
		}
	}
}

type tmpStruct struct {
	containerDeployTarget []string
	containerName         string
	containerImage        string
	templateEnvSlice      []string
	containerPortMap      map[string]string
	containerVolumeSlice  []string
	containerCmdSlice     []string
	containerNetworkMode  string
	capacityAdd           []string
	containerVolumeMap    map[string]string
	containerPrivileged   bool
	containerSpecialPort  string
	//


	tmpFs            map[string]string // key 是 卷  value 是参数
	logConfig        container.LogConfig
	privileged       bool // 是否开启特权模式
	extraHosts       []string

}

// 临时用日志配置
//var logtest = map[string]string{
//	"max-file": "1",
//	"max-size": "1024m",
//}

//var logcofig = container.LogConfig{"json-file", logtest}

func getEnvFromFile(envFile string) []string {
	var envs []string
	// 接收文件  返回 字符串类型的切片
	re := regexp.MustCompile(`.*=.*`) //匹配包含等号的行
	f, err := os.Open(envFile)
	if err != nil {
		fmt.Println("打开文件出错", err.Error())
	}
	//读取文件 建立缓冲区，把文件内容放到缓冲区中
	buf := bufio.NewReader(f)
	for {
		//遇到\n结束读取
		b, errR := buf.ReadBytes('\n')
		if errR != nil {
			if errR == io.EOF {
				break
			}
			//fmt.Println(errR.Error())
		}
		result := string(re.FindString(string(b)))
		if result != "" { // 匹配后的处理
			if !strings.HasPrefix(result, "#") { // 只处理不以#开头的行
				str := strings.Replace(result, " ", "", -1) //去掉空格
				// 以=为分割 分割字符串 返回字符切片
				arr := strings.Split(str, "=") //以等号分割字符串
				// 判断arr[1]中是否有#号
				arr1 := strings.Split(arr[1], "#")
				//fmt.Println("键：",arr[0],"值：",arr1[0])
				// pound key
				envs = append(envs, arr[0]+"="+arr1[0])
			}
		}
	}
	return envs
}

// 读取docker-compose 配置文件 找到env file 和 其中单独设置的 env
// 存储从docker-compose 文件读取的数据
type ymlConfig struct {
	name        string
	image       string
	portSlice   []string
	volumeSlice []string
	//envFileSlice     []string
	environmentSlice []string
	ports            []string
	portMap          map[string]string //   key 是外部端口 value是内部端口
	volumeMap        map[string]string //  key 是外部路径 value是内部路径
	cmd              []string          // 存储 传入的cmd 指令
	networkMode      string            // 网络模式 默认为bridge
	deployHost       string            // 部署到哪个服务器上
	capAdd           []string          //
	tmpFs            map[string]string // key 是 卷  value 是参数
	logConfig        container.LogConfig
	privileged       bool // 是否开启特权模式
	extraHosts       []string
}

type ymlTmpConfig struct {
	deployTarget                      []string
	supportMultiInstancesOnSingleHost bool // 是否支持单机多实例

	name        string
	image       string
	portSlice   []string
	volumeSlice []string
	//envFileSlice     []string
	environmentSlice []string
	ports            []string
	portMap          map[string]string //   key 是外部端口 value是内部端口
	volumeMap        map[string]string //  key 是外部路径 value是内部路径
	cmd              []string          // 存储 传入的cmd 指令
	networkMode      string            // 网络模式 默认为bridge
	deployHost       string            // 部署到哪个服务器上
	capAdd           []string          //
	tmpFs            map[string]string // key 是 卷  value 是参数
	logConfig        container.LogConfig
	privileged       bool // 是否开启特权模式
	extraHosts       []string
}

func ifSupportMultiInstancesOnSingleHost(networkMode string, portConfigSlice []string) bool {
	// 在 bridge 模式下 只有符合一定条件 才能支持 单机多实例部署
	// 支持 单机多实例部署的 网络配置条件，
	// 1 bridge 或者是空 ""   2 端口配置仅有一行  3 端口配置没有端口段配置 即不包含 -
	if (networkMode == "bridge" || networkMode == "") && (len(portConfigSlice) == 1) && (!strings.Contains(portConfigSlice[0], "-")) {

		return true

	} else {
		return false

	}

}

var ymlConfigSlice []ymlConfig

func tconfig(hconfig *viper.Viper) {
	//svc := hconfig.GetStringMap("services")
}

func GetConfigFromYml() {
	//dockerCompose := appName + "-docker-compose"

	config := viper.New()
	config.AddConfigPath(dockerCompsoeFileDir)                                    //设置读取的文件路径
	//config.SetConfigName(strings.Replace(dockerComposeFileName, ".yaml", "", -1)) //设置读取的文件名  去掉yaml 后缀


	noyaml:=strings.Replace(dockerComposeFileName, ".yaml", "", -1) // 去掉yaml
	noyml:=strings.Replace(noyaml,".yml","",-1)   // 去掉yml


	config.SetConfigName(noyml)



	config.SetConfigType("yaml")                                                  //设置文件的类型
	//尝试进行配置读取
	if err := config.ReadInConfig(); err != nil {
		panic(err)
	}
	tconfig(config)
	svc := config.GetStringMap("services")
	// 优化开始处
	for svcName, _ := range svc {
		// 构造一个临时结构体 作为参数传给处理函数
		tmpConfig := ymlTmpConfig{}
		quick(tmpConfig)

		//获取网络类型 网络类型决定了是否可以做 单机多实例， 只有是bridge 才可以做单机多实例部署
		var multiInstancesOnSingleHost bool
		// 网络类型 决定了 部署目标 的内容
		networkMode := config.GetString("services." + svcName + ".network_mode")
		portConfigSlice := config.GetStringSlice("services." + svcName + ".ports")
		multiInstancesOnSingleHost = ifSupportMultiInstancesOnSingleHost(networkMode, portConfigSlice)
		// 获取 引用的env 文件
		envFile := config.GetStringSlice("services." + svcName + ".env_file")
		// 获取 在docker-compose 文件中声明的环境变量  envs 此后还会存放来自 envFile中的值
		envs := config.GetStringSlice("services." + svcName + ".environment")
		templateEnvSlice := getTemplateEnvironmentSlice(envs, envFile)
		/////////////
		//  获取 targetSlice
		/////////////
		///#####################
		portSlice := config.GetStringSlice("services." + svcName + ".ports")
		//#############  获取port 相关初始配置
		volumeSlice := config.GetStringSlice("services." + svcName + ".volumes")
		//fmt.Println("svcVolumeMap",svcVolumeMap)
		//#############  获取vol 相关初始配置
		name := config.GetString("services." + svcName + ".container_name")
		image := config.GetString("services." + svcName + ".image")
		cmdSlice := config.GetStringSlice("services." + svcName + ".command")
		networkmode := config.GetString("services." + svcName + ".network_mode")
		cadadd := config.GetStringSlice("services." + svcName + ".cap_add")
		tmpfsSlice := config.GetStringSlice("services." + svcName + ".tmpfs")
		//dnsConfigSlice := config.GetStringSlice("services." + svcName + ".extra_hosts")
		//logConfigOptionMap := config.GetStringMap("services." + svcName + ".logging" + ".options")
		//logConfigDriver := config.GetString("services." + svcName + ".logging" + ".driver")
		privilegedStr := config.GetString("services." + svcName + ".Privileged")


		fmt.Println("获取tmpfsslice",tmpfsSlice)

		//newTmpfs(tmpfsSlice)

		// ExtraHosts  域名解析参数

		ExtraHosts := config.GetStringSlice("services." + svcName + ".extra_hosts")

		// 日志参数配置

		// 获取日志驱动类型
		logDriver := config.GetString("services." + svcName + ".logging" + ".driver") // 默认日志驱动是 json-file
		logConfigOptionMap := config.GetStringMapString("services." + svcName + ".logging" + ".options")
		var logConfig = container.LogConfig{
			Type:   logDriver,
			Config: logConfigOptionMap,
		}

		//   dockerComposeConfig 测试ok  确实tmpfs 支持
		fmt.Println("获取tmpfs",newTmpfs(tmpfsSlice))
		//dockerComposeConfig(getTarget(templateEnvSlice, multiInstancesOnSingleHost), name, image, templateEnvSlice, getPortMap(portSlice), volumeSlice, cmdSlice, networkmode, cadadd, getVolumeMap(volumeSlice), ifPrivileged(privilegedStr), getSpecialPort(portSlice), ExtraHosts, logConfig,newTmpfs(tmpfsSlice))

		if 1 < 2 {

			// 测试 使用 结构 传递参数
			a := &tmpStruct{
				containerDeployTarget: getTarget(templateEnvSlice, multiInstancesOnSingleHost),
				containerName:         name,
				containerImage:        image,
				templateEnvSlice:      templateEnvSlice,
				containerPortMap:      getPortMap(portSlice),
				containerVolumeSlice:  volumeSlice,
				containerCmdSlice:     cmdSlice,
				containerNetworkMode:  networkmode,
				capacityAdd:           cadadd,
				containerVolumeMap:    getVolumeMap(volumeSlice),
				containerPrivileged:   ifPrivileged(privilegedStr),
				containerSpecialPort:  getSpecialPort(portSlice),
				// 2022 04 14 add new
				logConfig: logConfig,
				extraHosts: ExtraHosts,
				tmpFs: 	newTmpfs(tmpfsSlice),


			}
			a.containerConfigFactory()
		}
	}
}

// 处理 ymlconfigSlice
func outConfig() { //装填 docker api 需要的元数据，并使用此数据 启动容器
	for _, ymlC := range ymlConfigSlice {

		fmt.Println("ymlC.volumeSlice", ymlC.volumeSlice)

		var portSet = make(nat.PortSet) // 单个容器 的portSet
		var portMap = make(nat.PortMap)
		for outPort, inPort := range ymlC.portMap {
			port, err := nat.NewPort("tcp", inPort)
			if err != nil {
				fmt.Println(err)
			}
			// 构造portSet
			portSet[port] = struct{}{}
			// 构造 nat.PortMap
			portMap[port] = []nat.PortBinding{{
				HostIP:   "0.0.0.0", //容器监听地址
				HostPort: outPort,   //容器外端口
			}}
		}

		// tmpfs test data
		tmpfs := make(map[string]string)
		tmpfs["/usr/local/xxx"] = "size=64m"

		config := &container.Config{
			//Volumes: volume,
			Volumes:      getContainerConfigVolMap(ymlC.volumeMap),
			Image:        ymlC.image,
			Tty:          false,
			Env:          ymlC.environmentSlice,
			ExposedPorts: portSet,
			Cmd:          ymlC.cmd,
		}
		// 构造容器配置 hostconfig
		var hosts []string
		hosts = append(hosts, "yyy.xxx.cn:100.01.101.90")

		//
		hostConfig := &container.HostConfig{
			ExtraHosts: ymlC.extraHosts,
			//Binds:      newBindsFromVolumeMap(ymlC.volumeMap),
			Binds: newBindsFromVolumeSlice(ymlC.volumeSlice),
			Tmpfs: ymlC.tmpFs,
			Privileged:   ymlC.privileged,
			CapAdd:       ymlC.capAdd,
			PortBindings: portMap,
			NetworkMode:  container.NetworkMode(ymlC.networkMode),
			//Mounts:       getMountSlice(ymlC.volumeMap), //   测试ok
			//Mounts: geVolSlice1(),
			LogConfig: ymlC.logConfig,
			RestartPolicy: container.RestartPolicy{ // 重启策略写死 没有使用docker-compose 文件
				Name:              "always",
				MaximumRetryCount: 0,
			},
		}
		deploy(ymlC.deployHost, config, hostConfig, ymlC.name)
	}
}

func deploy(host string, config *container.Config, hostConfig *container.HostConfig, name string) {

	// 临时定义一个默认host 仅用于用于测试
	if host == "localhost" {
		host = "172.16.100.3"
	}
	// 创建容器 容器名字  容器内端口 镜像名字 目标宿主机 环境变量

	ctx := context.Background()

	var cli *client.Client

	if certPath == "" {

		fmt.Println("证书路径为空")
		connectUrl := "http://" + host + ":2375"
		var err error
		cli, err = client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.38"), client.WithHost(connectUrl))

		defer cli.Close()
		if err != nil {
			panic(err)
		}

	} else {

		connectUrl := "http://" + host + ":2376"
		var err error
		cli, err = client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.38"), client.WithHost(connectUrl), client.WithTLSClientConfig(certPath+"/ca.pem", certPath+"/client-certs/cert.pem", certPath+"/client-certs/key.pem"))
		defer cli.Close()
		if err != nil {
			panic(err)
		}

	}

	//

	// 本地测试  无证书
	//connectUrl := "http://" + host + ":2375"
	//cli, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.38"), client.WithHost(connectUrl))

	// create 之前先删除
	// 判断当前是否有运行 同名 容器

	currentImageVersion := getVersionByName(cli, name)

	if currentImageVersion != config.Image { // 如果当前运行的镜像 不等于目标镜像  就尝试创建容器  //  currentImageVersion 为空说明  指定容器名字的容器没有运行，同样需要创建
		//

		fmt.Println("指定的镜像版本不一致 开始创建容器")

		// 先当当前服务器是否有指定镜像，如果没有才拉取

		// 拉取镜像
		pullImage(cli, config.Image)
		err := pullImage(cli, config.Image)


		if err != nil {
			fmt.Println("err", err)
		}

		// 删除指定名字的容器
		rebuild(cli, name)
		//

		resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, name)
		if err != nil {
			fmt.Println("name", name, "创建容器失败")
			panic(err)
		}

		if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
			fmt.Println("name", name, "启动容器失败")

			panic(err)
		}

		fmt.Println(name, host, "部署成功")

	}

}

func getExtraEvnSlice(envs []string) []string {
	// 存储 extra_env
	var extraEnvSlice []string
	// 获取 extra_env 的内容
	for _, i := range envs {
		if strings.Contains(i, "extra_env") { // 判断是否包含  extra_env
			extraStr := strings.Split(i, "=") //以等号分割字符串
			//os.Exit(0)
			if extraStr[0] == "extra_env" {
				extraEnv := strings.Split(i, `"`)
				//fmt.Println(extraEnv[1], "退出")
				noQuotationHosts := extraEnv[1]
				noLeftSquareBrackets := strings.Trim(noQuotationHosts, `[`)
				noSquareBrackets := strings.Trim(noLeftSquareBrackets, `]`)
				//fmt.Println("去掉双引号和方括号 EXTRA_ENV ", noSquareBrackets)
				//查看是否包含 逗号
				if strings.Contains(noSquareBrackets, ",") {
					// 如果包含就 用逗号分割
					extraEnv := strings.Split(noSquareBrackets, ",")
					for _, k := range extraEnv {
						//fmt.Println("分割后变量",k)
						//fmt.Println("EXTRA_ENV_xxx",k)
						extraEnvSlice = append(extraEnvSlice, k)
					}
				} else { // 不包含逗号
					//fmt.Println("EXTRA_ENV_xxx",noSquareBrackets)
					extraEnvSlice = append(extraEnvSlice, noSquareBrackets)
				}
			}
		}
	}
	return extraEnvSlice
}

func getTarget(envSlice []string, multiInstances bool) (target []string) {

	var targetSlice []string

	for _, env := range envSlice {
		// 获取 targetSlice
		if strings.Contains(env, "extra_hosts") {

			// "[172.16.100.3:3052-3057,172.16.100.4:3051-3055,172.16.100.5:3059]"
			extraStr := strings.Split(env, "=") //以等号分割字符串
			//fmt.Println("extra_hosts_slice",extraStr)
			if extraStr[0] == "extra_hosts" {
				noQuotationHosts := strings.Trim(extraStr[1], `"`)
				noLeftSquareBrackets := strings.Trim(noQuotationHosts, `[`)
				noRightSquareBrackets := strings.Trim(noLeftSquareBrackets, `]`)
				//hostStringSlice := strings.Split(noRightSquareBrackets, ",")
				// 对元素进行去重
				hostStringSlice := removeDuplicateElement(strings.Split(noRightSquareBrackets, ","))
				// 172.16.100.3:3052-3057,172.16.100.4:3051-3055,172.16.100.5:3059
				for _, hostAndPortSection := range hostStringSlice { // 需要去重
					if !strings.Contains(hostAndPortSection, ":") {
						// 如果不存在冒号 直接打印出ip
						targetSlice = append(targetSlice, hostAndPortSection+":"+"0")
					} else {
						//  字符存在冒号 只要存在冒号就说明有端口相关设置  只有支持单机多实例部署的网络配置 才可进行后续内容
						if multiInstances == true {
							// 先查判断大的前提 网络上要支持单机多实例
							hostAndPortSectionStrSlice := strings.Split(hostAndPortSection, ":")
							host := hostAndPortSectionStrSlice[0]
							// 如果不支持单机多实例，应当只有 host 没有端口设置
							PortSection := hostAndPortSectionStrSlice[1]
							if !strings.Contains(PortSection, "-") {
								//如果 端口字段不存在 -
								targetSlice = append(targetSlice, host+":"+PortSection)
							} else {
								portStrSlice := strings.Split(PortSection, "-")
								startPortStr := portStrSlice[0]
								endPortStr := portStrSlice[1]
								startPort, err := strconv.Atoi(startPortStr)
								if err != nil {
									fmt.Println(err)
								}
								endPort, err := strconv.Atoi(endPortStr)
								if err != nil {
									fmt.Println(err)
								}
								if endPort > startPort {
									for i := startPort; i <= endPort; i++ {
										targetSlice = append(targetSlice, host+":"+strconv.Itoa(i))
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return targetSlice

}

func getTemplateEnvSlice(originalEvnSlice, extraEvnSlice []string) []string {
	envTemplateSlice := originalEvnSlice
	for _, v := range extraEvnSlice { // 遍历扩展变量
		//fmt.Println(k,v)
		envKey := (strings.Split(v, "="))[0]           // 扩展变量 key
		envValue := (strings.Split(v, "="))[1]         // 扩展变量 值
		for i, originalEvn := range envTemplateSlice { // 变量原始变量
			key := (strings.Split(originalEvn, "="))[0]
			if key == envKey { // 如果 原始变量 和 扩展变量的 key 一样 就改成原始变量的值
				envTemplateSlice[i] = envKey + "=" + envValue
			}
			// 如果在扩展变量里存在 但是在 原始变量里不存在的 就将整个 扩展变量追加到原始变量
		}
	}
	// 将原始变量里的所有key 放到 temMap里
	tempMap := map[string]struct{}{}
	for _, value := range envTemplateSlice {
		key := (strings.Split(value, "="))[0]
		tempMap[key] = struct{}{}
	}
	// 遍历扩展变量 获取 key  判断是否存在于tempMap 不存在就追加进去
	for _, item := range extraEvnSlice {
		envKey := (strings.Split(item, "="))[0] // 扩展变量 key
		if _, ok := tempMap[envKey]; !ok {      //如果字典中找不到元素，ok=false，!ok为true，就往切片中append元素
			envTemplateSlice = append(envTemplateSlice, item)
		}
	}
	return envTemplateSlice
}

func dockerComposeConfig(targetSlice []string, name string, image string, environmentSlice []string, portMap map[string]string, volumeSlice []string, cmdSlice []string, networkmode string, cadadd []string, volMap map[string]string, isPrivileged bool, specialPort string, extraHosts []string, logConfig container.LogConfig,tmpFs map[string]string) {
	fmt.Println("specialPort", specialPort)

	if len(targetSlice) == 0 { // 如果   targetSlice 长度为0 意味着是一个普通的 docker-compsoe 配置
		ymlconfig := ymlConfig{
			volumeSlice:      volumeSlice,
			name:             name,
			image:            image,
			environmentSlice: environmentSlice,
			portMap:          portMap,
			volumeMap:        volMap,
			cmd:              cmdSlice,
			networkMode:      networkmode,
			deployHost:       "localhost",
			capAdd:           cadadd,
			privileged:       isPrivileged,
			extraHosts:       extraHosts,
			logConfig:        logConfig,
			tmpFs: tmpFs,
		}
		ymlConfigSlice = append(ymlConfigSlice, ymlconfig)
	} else {
		for _, target := range targetSlice {
			ip := (strings.Split(target, ":"))[0]
			port := (strings.Split(target, ":"))[1]
			if port == "0" {
				// 一个机器上 只跑一个服务的实例
				fmt.Println("portMap", portMap)
				envs := modifyEnv(environmentSlice, ip, specialPort) // 如果port 等于0  意味着 在一个机器上只跑一个实例，代表不用修改端口相关配置，这个port应该取原始端口配置第一个容器外端口
				fmt.Println(specialPort, volMap, "volumeMap")
				ymlconfig := ymlConfig{
					volumeSlice:      volumeSlice,
					name:             name,
					image:            image,
					environmentSlice: envs,
					portMap:          portMap,
					volumeMap:        volMap,
					cmd:              cmdSlice,
					networkMode:      networkmode,
					deployHost:       ip,
					capAdd:           cadadd,
					privileged:       isPrivileged,
					extraHosts:       extraHosts,
					logConfig:        logConfig,
					tmpFs: tmpFs,

				}
				ymlConfigSlice = append(ymlConfigSlice, ymlconfig)
			} else {
				// 部署形式为单机多实例    判断手段  port 不为0  port 不为零 说明要修改端口号，端口配置只能有一行 且不能是端口段
				//fmt.Println("单机多实例",name)
				var inPort string
				for _, in := range portMap {
					inPort = in
				}
				PortMap := make(map[string]string)
				PortMap[port] = inPort
				// 处理 单机多实例下的端口 ############
				// 处理单机多实例下的 volume
				svcVolumeMap := volumeSlice2Map(volumeSlice, port)
				//fmt.Println(svcVolumeMap)
				// 处理单机多实例下的 envs
				envsTem := environmentSlice
				envs := modifyEnv(envsTem, ip, port)
				ymlconfig := ymlConfig{
					volumeSlice: modifyVolumeSlice(volumeSlice, port),
					//volumeSlice: volumeSlice,
					name:             name + port,
					image:            image,
					environmentSlice: envs,
					portMap:          PortMap,
					volumeMap:        svcVolumeMap,
					//volumeMap:        volMap,
					cmd:         cmdSlice,
					networkMode: networkmode,
					deployHost:  ip,
					capAdd:      cadadd,
					privileged:  isPrivileged,
					extraHosts:  extraHosts,
					logConfig:   logConfig,
					tmpFs: tmpFs,

				}
				ymlConfigSlice = append(ymlConfigSlice, ymlconfig)
			}
		}
	}
}

// 对slice 中的元素进行去重
func removeDuplicateElement(originalSlice []string) []string {
	result := make([]string, 0, len(originalSlice))
	tempMap := map[string]struct{}{}
	for _, item := range originalSlice {
		if _, ok := tempMap[item]; !ok { //如果字典中找不到元素，ok=false，!ok为true，就往切片中append元素
			tempMap[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

func volumeSlice2Map(volumeSlice []string, portStr string) (svcVolumeMap map[string]string) {
	svcVolumeMap = make(map[string]string)
	//volumeSlice := config.GetStringSlice("services." + svcName + ".volumes")
	for _, v := range volumeSlice {
		//fmt.Println(i, v)
		arr := strings.Split(v, ":")
		//fmt.Println("容器外路径", arr[0], "容器内路径", arr[1])
		outPath := strings.Replace(arr[0], "[outPort]", portStr, -1) //去掉空格
		svcVolumeMap[outPath] = arr[1]
	}
	return svcVolumeMap
}

func modifyEnv(envsTemplate []string, ip string, port string) []string {
	// 获取extraEnv
	extraEnv := getExtraEvnSlice(envsTemplate)
	// 将extraEnv 的 key 存入 extraEnvMap
	extraEnvMap := make(map[string]struct{})
	for _, v := range extraEnv {
		arr := strings.Split(v, "=")
		extraEnvKey := arr[0]
		extraEnvMap[extraEnvKey] = struct{}{}
	}
	// 只修改 等号右边 包含特殊字符的值
	var envSlice []string
	for _, envStr := range envsTemplate {
		//只修复extra_env里定义的变量，未定义的不修改
		// 排除 extra_env 和 extra_hosts
		if !strings.Contains(envStr, "extra_env") && !strings.Contains(envStr, "extra_hosts") {
			if checkExtraEnv(extraEnvMap, getEnvKey(envStr)) {
				//result := strings.Replace(strings.Replace(envStr, "host", ip, -1), "outPort", port, -1)
				result := strings.Replace(strings.Replace(strings.Replace(envStr, "host", ip, -1), "outPort", port, -1), "ip_suffix", getIpSuffixStr(ip), -1)
				envSlice = append(envSlice, result)
			} else {
				// 不在extraEnv里声明的变量直接添加
				envSlice = append(envSlice, envStr)
			}
		}
	}
	return envSlice
}

func newTmpfs(tmpfsSlice []string) map[string]string {
	tmpfs := make(map[string]string)
	// 判断是否有冒号 有冒号代表有挂载参数
	for _, v := range tmpfsSlice {
		tmpfsStr:= strings.Split(v, ":")
		tmpfs[tmpfsStr[0]]=tmpfsStr[1]
	}
	return tmpfs
}

func checkExtraEnv(extraEnvMap map[string]struct{}, key string) bool {
	_, is := extraEnvMap[key]
	return is
}

func getEnvKey(envStr string) (keyName string) {
	if strings.Contains(envStr, "=") {
		arr := strings.Split(envStr, "=")
		if len(arr) >= 1 {
			return arr[0]
		}
	}
	return ""
}

func getIpSuffixStr(ipStr string) string {
	ipSuffix := strings.Split(ipStr, ".")
	if len(ipSuffix) == 4 {
		return ipSuffix[3]
	}
	return ""
}

func quick(ymlTmpConfig) {
}

func getSpecialPort(portSlice []string) string {
	if len(portSlice) >= 1 {
		if len(strings.Split(portSlice[0], ":")) >= 1 {
			return strings.Split(portSlice[0], ":")[0]
		}
	}
	return ""
}

func getTemplateEnvironmentSlice(envSlice, envFileSlice []string) []string {
	for _, file := range envFileSlice {
		filename := filepath.Base(file)
		env_t := getEnvFromFile(dockerCompsoeFileDir + "/env/" + filename) // 在指定路径下搜索envfile 文件
		envSlice = append(envSlice, env_t...)
	}
	// 对全量的envs进行排序
	sort.Strings(envSlice)
	//从extra_env 字段中获取 具有特殊含义的变量 存于dynamicEvnSlice中
	dynamicEvnSlice := getExtraEvnSlice(envSlice)
	templateEnvSlice := getTemplateEnvSlice(envSlice, dynamicEvnSlice)
	sort.Strings(templateEnvSlice) // 生成一个环境变量的模版
	return templateEnvSlice
}

func ifPrivileged(privilegeStr string) bool {
	if privilegeStr == "true" {
		return true
	} else {
		return false
	}
}

func getVolumeMap(volumeSlice []string) map[string]string {
	svcVolumeMap := make(map[string]string)
	for _, v := range volumeSlice {
		//fmt.Println(i, v)
		arr := strings.Split(v, ":")
		//fmt.Println("容器外路径", arr[0], "容器内路径", arr[1])
		svcVolumeMap[arr[0]] = arr[1]
	}
	return svcVolumeMap
}

func getPortMap(portSlice []string) map[string]string {
	svcPortMap := make(map[string]string)
	for _, v := range portSlice {
		//fmt.Println(i, v)
		arr := strings.Split(v, ":")
		//fmt.Println("分割后的端口", arr[0], arr[1])
		svcPortMap[arr[0]] = arr[1]
		// 如果存储的是端口段
	}
	return svcPortMap
}

//  mount.TypeBind
func getMountSlice(volumeMap map[string]string) []mount.Mount {
	mountSlice := []mount.Mount{}
	//fmt.Println("ymlC.volumeMap",ymlC.volumeMap)
	for outPath, inPath := range volumeMap {
		//fmt.Println("挂载信息:", "容器外路径", outPath, "容器内路径", inPath)
		mountConfig := mount.Mount{
			Type:   mount.TypeBind, //   tested
			Source: outPath,
			Target: inPath,
		}
		mountSlice = append(mountSlice, mountConfig)
	}
	return mountSlice
}

// test docker-compsoe volume

// create volume   mount.TypeVolume ok

func geVolSlice() []mount.Mount {
	mountSlice := []mount.Mount{}
	mountConfig := mount.Mount{
		Type:   mount.TypeVolume,
		Source: "myvol",
		Target: "/rootfs",
		VolumeOptions: &mount.VolumeOptions{
			DriverConfig: &mount.Driver{
				Name: "local",
			},
		},
	}
	mountSlice = append(mountSlice, mountConfig)

	return mountSlice
}

func getContainerConfigVolMap(volMap map[string]string) map[string]struct{} {
	ContainerConfigVolMap := make(map[string]struct{})
	for _, vol := range volMap {
		ContainerConfigVolMap[vol] = struct{}{}
	}
	return ContainerConfigVolMap
}

func newBindsFromVolumeMap(volMap map[string]string) []string {
	fmt.Println("newBinds", volMap)
	var binds []string
	for outPath, inPath := range volMap {
		binds = append(binds, outPath+":"+inPath+":ro")
	}
	return binds
}

func newBindsFromVolumeSlice(volSlice []string) []string {
	var binds []string

	for _, vol := range volSlice {
		temStr := strings.Split(vol, ":")
		if len(temStr) == 2 {
			binds = append(binds, vol+":rw")
		}
		if len(temStr) == 3 {
			binds = append(binds, vol)

		}
	}
	return binds
}

func modifyVolumeSlice(a []string, b string) []string {
	var c []string
	//volumeSlice := config.GetStringSlice("services." + svcName + ".volumes")
	for _, v := range a {
		d := strings.Replace(v, "[outPort]", b, -1) //去掉空格
		c = append(c, d)
	}

	fmt.Println("xxxx", c)
	return c
}

func httpServer() {

}

func getVersionByName(cli *client.Client, name string) (currentVersion string) {

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}
	for _, container := range containers {
		if container.Names[0] == "/"+name {
			return container.Image
		}
	}

	return ""
}

func pullImage(cli *client.Client, image string) error {

	fmt.Println("部署目标镜像：", image)

	// 不存在才拉取

	ctx := context.Background()

	images, err := cli.ImageList(ctx, types.ImageListOptions{All: true})

	if err != nil {

	}

	for _, localImage := range images {

		fmt.Println("image.RepoTags", localImage.RepoTags)

		for _, i := range localImage.RepoTags {
			if i == image {
				fmt.Println("本地存在此版本，跳过拉取")
				return nil

			}

		}
	}

	//reader, err := cli.ImagePull(ctx, iname, types.ImagePullOptions{RegistryAuth: authStr})
	reader, err := cli.ImagePull(ctx, image, getAuth(image))

	if err != nil {
		//fmt.Println("err",err)
		fmt.Println("认证镜像仓库失败", err)
		return err
	}
	defer reader.Close()

	//io.Copy(os.Stdout, reader)

	io.Copy(ioutil.Discard, reader)

	fmt.Println("拉取镜像")

	//var out io.Writer
	//out, _ = os.OpenFile("/tmp/pull.txt", os.O_RDWR, 0666)
	//touch /tmp/pull.txt   chmod 777 /tmp/pull.txt
	//	io.Copy(out, reader)
	fmt.Println("拉取镜像:success")

	return nil
}

func rebuild(cli *client.Client, name string) {
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}
	for _, container := range containers {
		if container.Names[0] == "/"+name {
			//			fmt.Println("xx")

			if err := cli.ContainerStop(context.Background(), container.ID, nil); err != nil {
				panic(err)

				if err != nil {

					err := cli.ContainerRemove(context.Background(), container.ID, types.ContainerRemoveOptions{})
					if err != nil {
						fmt.Println(err)
					}
				}
			}
			fmt.Println("stop", "Success")
			err := cli.ContainerRemove(context.Background(), container.ID, types.ContainerRemoveOptions{})
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println("rm success")
		}
	}

}

func getAuth(imageStr string) types.ImagePullOptions {
	if strings.Contains(imageStr, ":") {
		strSlice := strings.Split(imageStr, ":")
		if !strings.Contains(strSlice[0], "/") {
			return types.ImagePullOptions{}
		}
	}
	authConfig := types.AuthConfig{
		Username: "xxx",
		Password: "xxx",
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		panic(err)
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)
	return types.ImagePullOptions{RegistryAuth: authStr}
}

func cmd() (cert string, filePath string) {
	// 定义几个变量，用于接收命令行的参数值
	var certs string
	var file string
	var auth string


	// &user 就是接收用户命令行中输入的 -u 后面的参数值
	// "u" ,就是 -u 指定参数
	// "" , 默认值
	// "用户名,默认为空" 说明
	flag.StringVar(&certs, "c", "", "证书路径")
	flag.StringVar(&file, "f", "", "docker-compose 文件路径")
	flag.StringVar(&auth, "a", "./auth.json", "仓库认证文件")

	// 这里有一个非常重要的操作,转换， 必须调用该方法
	flag.Parse()
	// 输出结果
	if !Exists(file) {
		fmt.Println(file, "不存在")
		os.Exit(0)

	}

	if certs != "" && !Exists(certs) {
		fmt.Println(certs, "不存在")
		os.Exit(0)

	}

	return certs, file

}

func Exists(path string) bool {
	_, err := os.Stat(path) //os.Stat获取文件信息
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}
