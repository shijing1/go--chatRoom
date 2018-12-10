package main

import (
	"net"
	"fmt"
	"strings"
	"time"
)
// 创建用户结构体类型！
type Client struct {
	C chan string
	Name string
	Addr string
}

// 创建全局map，存储在线用户
var onlineMap map[string]Client

// 创建全局 channel 传递用户消息。
var    message = make(chan string)

func WriteMsgToClient(clnt Client, conn net.Conn)  {
	// 监听 用户自带Channel 上是否有消息。
	for msg := range clnt.C {
		conn.Write([]byte(msg + "\n"))
	}
}

func MakeMsg(clnt Client, msg string) (buf string) {
	buf = "[" + clnt.Addr + "]" + clnt.Name + ": " + msg
	return
}

func HandlerConnect(conn net.Conn)  {
	defer conn.Close()
	// 创建channel 判断，用户是否活跃。
	hasData := make(chan bool)

	// 获取用户 网络地址 IP+port
	netAddr := conn.RemoteAddr().String()
	// 创建新连接用户的 结构体. 默认用户是 IP+port
	clnt := Client{make(chan string), netAddr, netAddr}

	// 将新连接用户，添加到在线用户map中. key: IP+port value：client
	onlineMap[netAddr] = clnt

	// 创建专门用来给当前 用户发送消息的 go 程
	go WriteMsgToClient(clnt, conn)

	// 发送 用户上线消息到 全局channel 中
	//message <- "[" + netAddr + "]" + clnt.Name + "login"
	message <- MakeMsg(clnt, "login")

	// 创建一个 channel ， 用来判断用退出状态
	isQuit := make(chan bool)

	// 创建一个匿名 go 程， 专门处理用户发送的消息。
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n == 0 {
				isQuit <- true
				fmt.Printf("检测到客户端:%s退出\n", clnt.Name)
				return
			}
			if err != nil {
				fmt.Println("conn.Read err:", err)
				return
			}
			// 将读到的用户消息，保存到msg中，string 类型
			msg := string(buf[:n-1])

			// 提取在线用户列表
			if msg == "who" && len(msg) == 3 {
				conn.Write([]byte("online user list:\n"))
				// 遍历当前 map ，获取在线用户
				for _, user := range onlineMap {
					userInfo := user.Addr + ":" + user.Name + "\n"
					conn.Write([]byte(userInfo))
				}
				// 判断用户发送了 改名 命令
			} else if len(msg) >=8 && msg[:6] == "rename" {		// rename|
				newName := strings.Split(msg, "|")[1]		// msg[8:]
				clnt.Name = newName								// 修改结构体成员name
				onlineMap[netAddr] = clnt						// 更新 onlineMap
				conn.Write([]byte("rename successful\n"))
			}else {
				// 将读到的用户消息，写入到message中。
				message <- MakeMsg(clnt, msg)
			}
			hasData <- true
		}
	}()

	// 保证 不退出
	for {
		// 监听 channel 上的数据流动
		select {
		case <-isQuit:
			delete(onlineMap, clnt.Addr)		// 将用户从 online移除
			message <- MakeMsg(clnt, "logout")   // 写入用户退出消息到全局channel
			return
		case <-hasData:
			// 什么都不做。 目的是重置 下面 case 的计时器。
		case <-time.After(time.Second * 60):
			delete(onlineMap, clnt.Addr)       // 将用户从 online移除
			message <- MakeMsg(clnt, "time out leaved") // 写入用户退出消息到全局channel
			return
		}
	}
}

func Manager()  {
	// 初始化 onlineMap
	onlineMap = make(map[string]Client)

	// 监听全局channel 中是否有数据, 有数据存储至 msg， 无数据阻塞。
	for {
		msg := <-message

		// 循环发送消息给 所有在线用户。要想执行，必须 msg := <-message 执行完， 解除阻塞。
		for _, clnt := range onlineMap {
			clnt.C <- msg
		}
	}
}

func main()  {
	// 创建监听套接字
	listener, err := net.Listen("tcp", "127.0.0.1:8000")
	if err != nil {
		fmt.Println("Listen err", err)
		return
	}
	defer listener.Close()

	// 创建管理者go程，管理map 和全局channel
	go Manager()

	// 循环监听客户端连接请求
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept err", err)
			return
		}
		// 启动go程处理客户端数据请求
		go HandlerConnect(conn)
	}
}
