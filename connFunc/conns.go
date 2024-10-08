package connFunc

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/BeefFurUtilDev/tinyRconClient/printUtil"
	"github.com/BeefFurUtilDev/tinyRconClient/types"
	"github.com/jltobler/go-rcon"
	"github.com/rs/zerolog"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// NewSession 建立一个新的RCON会话。
// 它尝试连接到一个Minecraft服务器，然后在一个循环中读取用户输入的命令并将其发送到服务器，直到会话被中断或用户决定退出。
// 参数:
//
//	clientSetup: 包含连接信息（地址、端口和密码）的结构体。
//
// 返回值:
//
//	错误: 如果在建立连接或执行命令时发生错误，则返回相应的错误。
func NewSession(clientSetup types.Client) (err error) {
	// 初始化日志输出格式和时间格式
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	log := zerolog.New(output).With().Timestamp().Logger()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Info().Msg("starting session...")

	// 尝试连接到RCON服务器
	conn, err := rcon.Dial("rcon://"+clientSetup.Addr+":"+strconv.Itoa(clientSetup.Port), clientSetup.Password)
	if err != nil {
		log.Error().AnErr("conn error:", err).Msgf("can't connect to server")
		return err
	}
	// 确保在函数结束时关闭连接
	defer func(conn *rcon.Conn) {
		_ = conn.Close()
	}(conn)
	// 初始化变量以读取标准输入和处理中断信号
	var stdInput string
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGTERM)
	scanner := bufio.NewScanner(os.Stdin)
	// 主循环：处理命令输入和中断信号
	for {
		select {
		case <-interruptChan:
			// 当收到中断信号时，退出循环
			fmt.Println("\nCaught ^C, exiting...")
			return nil
		default:
			// 打印提示符
			printUtil.PS1(clientSetup.Addr, clientSetup.Port)
			// 读取并处理用户输入
			if scanner.Scan() {
				stdInput = scanner.Text()
			} else {
				// 处理扫描错误
				err := scanner.Err()
				if err != nil {
					switch {
					case errors.Is(err, bufio.ErrTooLong):
						log.Error().Err(err).Msg("input too long")
					case err == io.EOF:
						log.Info().Msg("EOF detected, exiting...")
						return nil
					default:
						log.Error().AnErr("scan error:", err).Msg("can't read input")
					}
				}
			}

			// 处理空输入或exit命令
			if stdInput == "" {
				fmt.Println("")
				continue
			}
			if stdInput == "exit" || stdInput == "stop" {
				return nil
			}
			// 发送命令并处理结果
			result, err := conn.SendCommand(stdInput)
			switch {
			case err == nil:
				if result == "" {
					log.Info().Msg("no response.")
					continue
				}
			case errors.Is(err, errors.New("connection closed")):
				log.Error().Msg("connection closed, reconnecting...")
				for i := 3; i == 0 || err != nil; i-- {
					time.Sleep(time.Second * 5)
					log.Info().Msgf("retry num: %d, reconnecting in %d seconds...", i, 5)
					conn, err = rcon.Dial("rcon://"+clientSetup.Addr+":"+strconv.Itoa(clientSetup.Port), clientSetup.Password)
				}
				if err != nil {
					log.Error().AnErr("conn error:", err).Msgf("can't connect to server")
					func(conn *rcon.Conn) {
						_ = conn.Close()
					}(conn)
					break
				}
			}
			if err != nil {
				log.Error().AnErr("command error:", err).Msg("can't execute command")
				continue
			}
			log.Info().Msg(result)
		}
	}
}

// ExecCommand 执行服务器的RCON命令。
// 该函数通过RCON协议连接到服务器，并发送指定的命令，然后返回命令的结果或错误。
// 参数:
//   - clientSetup: 包含连接信息（地址、端口和密码）的客户端设置指针。
//   - cmd: 指向要发送的命令的指针。
//
// 返回值:
//   - string: 服务器对命令的响应结果。
//   - error: 如果连接、发送命令或连接关闭时发生错误，则返回该错误。
func ExecCommand(clientSetup *types.Client, cmd *string) (result string, err error) {
	// 设置日志输出格式和时间格式。
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	log := zerolog.New(output).With().Timestamp().Logger()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// 根据客户端设置，尝试建立与服务器的RCON连接。
	conn, err := rcon.Dial("rcon://"+(*clientSetup).Addr+":"+strconv.Itoa(clientSetup.Port), (*clientSetup).Password)
	if err != nil {
		// 如果连接失败，记录错误并返回。
		log.Error().AnErr("conn error:", err).Msgf("can't connect to server")
		return "", err
	}
	// 确保连接在函数返回前关闭。
	defer func(conn *rcon.Conn) {
		_ = conn.Close()
	}(conn)
	// 发送命令并接收结果。
	result, err = conn.SendCommand(*cmd)
	// 记录发送的命令。
	log.Info().Msgf("command: \"%s\" sended!", *cmd)
	if err != nil {
		// 如果发送命令时发生错误，记录错误。
		log.Error().AnErr("send command error:", err).Msgf("can't send command: %d", cmd)
	}
	if result == "" {
		// 如果命令的响应结果为空，记录警告信息。
		log.Warn().Msgf("response is empty!")
	}
	return
}
func ExecCommandWithInput(clientSetup *types.Client, input *chan string, outPut *chan string) (err error) {
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	log := zerolog.New(output).With().Timestamp().Logger()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	isOpen := true
	conn, err := rcon.Dial("rcon://"+(*clientSetup).Addr+":"+strconv.Itoa(clientSetup.Port), (*clientSetup).Password)
	if err != nil {
		log.Error().AnErr("conn error:", err).Msgf("can't connect to server")
		return err
	}
	defer func(conn *rcon.Conn) {
		_ = conn.Close()
	}(conn)
	for isOpen {
		val, ok := <-*input
		if !ok {
			isOpen = false
			err = errors.New("read channel data failed")
			log.Error().AnErr("read channel data failed", err).Msg("chan err!")
			continue
		}
		if val == "" {
			isOpen = false
		} else {
			result, err := conn.SendCommand(val)
			if err != nil {
				*outPut <- fmt.Sprintf("exec fail with: %s", err.Error())
				log.Error().AnErr("send command error:", err).Msgf("can't send command: %d", val)
			} else {
				*outPut <- fmt.Sprintf("command: \"%s\" sended!", val)
				*outPut <- result
				if result == "" {
					*outPut <- "response is empty!"
				}
			}
			continue
		}
	}
	return nil
}
