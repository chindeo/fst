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
var times = flag.Int("t", 50, "并发测试次数")
var timeout = flag.Int("timeout", 5, "超时时间，默认5s")
var s = flag.Int("s", 5, "每次测试连接保留时间，默认5s")

func main() {
	flag.Parse()

	ts := *times
	sleep := *s
	failed := make(chan error, ts)
	successed := make(chan time.Time, ts)
	defer close(failed)
	defer close(successed)
	for i := 0; i < ts; i++ {
		go func() {
			err := CheckFreeSwitch(*ip, sleep, *timeout)
			if err != nil {
				failed <- err
				return
			}
			successed <- time.Now()
		}()
	}

	fmt.Printf(`====>开始测试:
-服务地址： %s
-测试次数： %d
-单次测试时间： %d
-连接超时时间： %d
`, *ip, ts, sleep, *timeout)

	go func() {
		bar := progressbar.NewOptions(ts*sleep,
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
		for i := 0; i < ts*sleep; i++ {
			bar.Add(1)
			time.Sleep(1 * time.Second)
		}
	}()

	time.Sleep(time.Duration(ts*sleep+2) * time.Second)

	fmt.Printf("\n测试结果:\n")
	go func() {
		fmt.Printf("\n成功:【%d】 次\n", len(successed))
		for s := range successed {
			fmt.Printf("成功时间： %v\n", s)
		}
	}()

	go func() {
		fmt.Printf("\n失败:【%d】 次\n", len(failed))
		for f := range failed {
			fmt.Printf("失败原因：%v\n", f)
		}
	}()
	select {}
}

// 监控freeswitch 服务是否正常
// https://github.com/jart/gosip/blob/master/example/rawsip/rawsip_test.go
// 本地环境可以监控，医院环境无法实现，报错超时
func CheckFreeSwitch(raddr string, sleep, timeout int) error {
	conn, err := net.DialTimeout("udp", raddr, time.Second*time.Duration(timeout))
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
	time.Sleep(time.Duration(*s) * time.Second)
	return nil
}
