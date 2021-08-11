package main

import (
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/jart/gosip/util"
	"github.com/k0kubun/go-ansi"
	"github.com/schollz/progressbar/v3"
)

var ip = flag.String("ip", "", "需要测试的fs服务器地址")
var times = flag.Int("t", 50, "并发测试次数,最大100000")
var timeout = flag.Int64("timeout", 5, "超时时间，默认5s")
var s = flag.Int64("s", 5, "每次测试连接保留时间，默认5s")

func main() {
	flag.Parse()

	failed := make(chan error, 100000)
	successed := make(chan time.Time, 100000)
	for i := 0; i < *times; i++ {
		go func() {
			err := CheckFreeSwitch(*ip, *s)
			if err != nil {
				failed <- err
				return
			}
			successed <- time.Now()
		}()

	}

	fmt.Printf(`开始测试:
-服务地址： %s
-测试次数： %d
-单次测试时间： %d
-连接超时时间： %d`, *ip, *times, *s, *timeout)

	go func() {
		bar := progressbar.NewOptions(*times*5,
			progressbar.OptionSetWriter(ansi.NewAnsiStdout()),
			progressbar.OptionEnableColorCodes(true),
			// progressbar.OptionShowBytes(true),
			progressbar.OptionSetWidth(15),
			progressbar.OptionSetDescription("[cyan][1/1][reset] 服务测试中"),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "[green]=[reset]",
				SaucerHead:    "[green]>[reset]",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}))
		for i := 0; i < *times*5*2; i++ {
			bar.Add(1)
			time.Sleep(500 * time.Millisecond)
		}
	}()

	time.Sleep(time.Duration(*times*5) * time.Second)

	fmt.Printf("\n成功:%d 次\n", len(successed))

	fmt.Println("==================")

	fmt.Printf("\n失败:%d 次\n", len(failed))
	for i := 0; i < len(failed); i++ {
		fmt.Println(<-failed)
	}
}

// 监控freeswitch 服务是否正常
// https://github.com/jart/gosip/blob/master/example/rawsip/rawsip_test.go
// 本地环境可以监控，医院环境无法实现，报错超时
func CheckFreeSwitch(raddr string, sleep int64) error {
	conn, err := net.DialTimeout("udp", raddr, time.Second*time.Duration(*timeout))
	if err != nil {
		return fmt.Errorf("check freeswitch dail timeout")
	}
	defer conn.Close()

	laddr := conn.LocalAddr().String()
	cseq := util.GenerateCSeq()
	fromtag := util.GenerateTag()
	callid := util.GenerateCallID()
	packet := "" +
		"OPTIONS sip:1001@" + laddr + " SIP/2.0\r\n" +
		"Via: SIP/2.0/UDP " + laddr + "\r\n" +
		"Max-Forwards: 70\r\n" +
		"To: <sip:" + raddr + ">\r\n" +
		"From: <sip:" + laddr + ">;tag=" + fromtag + "\r\n" +
		"Call-ID: " + callid + "\r\n" +
		"CSeq: " + strconv.Itoa(cseq) + " OPTIONS\r\n" +
		"Contact: <sip:" + laddr + ">\r\n" +
		"User-Agent: pokémon/1.o\r\n" +
		"Accept: application/sdp\r\n" +
		"Content-Length: 0\r\n" +
		"\r\n"
	bpacket := []uint8(packet)
	amt, err := conn.Write(bpacket)
	if err != nil || amt != len(bpacket) {
		return fmt.Errorf("write %w", err)
	}
	err = conn.SetDeadline(time.Now().Add(time.Second * 1))
	if err != nil {
		return fmt.Errorf("SetDeadline %w", err)
	}

	buf := make([]byte, 2048)
	amt, err = conn.Read(buf)
	if err != nil {
		return fmt.Errorf("conn.Read %w", err)
	}
	response := buf[0:amt]
	msg := string(response)
	lines := strings.Split(msg, "\r\n")
	if lines[0] != "SIP/2.0 200 OK" && lines[0] != "SIP/2.0 482 Request merged" {
		fmt.Printf("not ok :[\n%s", msg)
		return fmt.Errorf("not ok :[\n%s", msg)
	}
	time.Sleep(time.Duration(sleep) * time.Second)
	return nil
}
