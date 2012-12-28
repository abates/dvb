package main

import (
	"fmt"
	"github.com/ziutek/dvb"
	"github.com/ziutek/dvb/linuxdvb/demux"
	"github.com/ziutek/dvb/linuxdvb/frontend"
	"github.com/ziutek/dvb/ts"
	"github.com/ziutek/sched"
	"github.com/ziutek/thread"
	"os"
	"runtime"
	"time"
)

const (
	adpath  = "/dev/dvb/adapter0"
	fepath  = adpath + "/frontend0"
	dmxpath = adpath + "/demux0"
	dvrpath = adpath + "/dvr0"
	freq    = 778 // MHz
	pcrpid  = 202
)

func checkErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if err != dvb.OverflowError && err != ts.SyncError {
			os.Exit(1)
		}
	}
}

type PCR struct {
	lastPCR  ts.PCR
	lastRead time.Time
	firstPCR time.Time

	cnt       uint32
	jitterSum time.Duration
	jitterMax time.Duration

	discard bool
}

func (p *PCR) reset() {
	p.cnt = 0
	p.jitterSum = 0
	p.jitterMax = 0
	p.discard = false
}

func (p *PCR) PrintReport() {
	p.discard = true
	cnt := time.Duration(p.cnt)
	fmt.Printf(
		"period: %s, jitter: avg=%s, max=%s\n",
		time.Now().Sub(p.firstPCR)/cnt, p.jitterSum/cnt, p.jitterMax,
	)
	p.reset()
}

func (p *PCR) Loop(dvr ts.PktReader) {
	runtime.LockOSThread()

	uid := os.Geteuid()
	if uid == 0 {
		t := thread.Current()
		fmt.Println("Setting realtime sheduling for thread:", t)
		p := sched.Param{
			Priority: sched.FIFO.MaxPriority(),
		}
		checkErr(t.SetSchedPolicy(sched.FIFO, &p))
	} else {
		fmt.Println(
			"Running without root privilages: realtime scheduling disabled",
		)
	}
	fmt.Println()

	pkt := ts.NewPkt()

	for {
		err := dvr.ReadPkt(pkt)
		now := time.Now()
		checkErr(err)

		if p.discard {
			continue
		}
		if !pkt.Flags().ContainsAF() {
			continue
		}
		af := pkt.AF()
		if !af.Flags().ContainsPCR() {
			continue
		}
		pcr := af.PCR()

		if p.cnt == 0 {
			p.firstPCR = now
		} else {
			pcrDiff := (pcr - p.lastPCR).Nanosec()
			readDiff := now.Sub(p.lastRead)
			var jitter time.Duration
			if pcrDiff > readDiff {
				jitter = pcrDiff - readDiff
			} else {
				jitter = readDiff - pcrDiff
			}
			p.jitterSum += jitter
			if p.jitterMax < jitter {
				p.jitterMax = jitter
			}
		}

		p.lastPCR = pcr
		p.lastRead = now
		p.cnt++

	}
}

func main() {
	fedev, err := frontend.Open(fepath)
	checkErr(err)
	fe := frontend.API3{fedev}

	feInfo, err := fe.Info()
	checkErr(err)
	fmt.Println("Frontend information\n")
	fmt.Println(feInfo)

	if feInfo.Type != frontend.DVBT {
		fmt.Fprintln(
			os.Stderr,
			"This application supports only DVB-T frontend.",
		)
		os.Exit(1)
	}

	fmt.Printf("Tuning to %d MHz...\n", uint(freq))
	feParam := frontend.DefaultParamDVBT(feInfo.Caps, "pl")
	feParam.Freq = freq * 1e6
	checkErr(fe.TuneDVBT(feParam))

	var ev frontend.EventDVBT
	for ev.Status&frontend.HasLock == 0 {
		checkErr(fe.GetEventDVBT(&ev))
		fmt.Println("FE status:", ev.Status)
	}
	fmt.Println()

	dmx := demux.Dev(dmxpath)
	filterParam := demux.StreamFilterParam{
		Pid:  pcrpid,
		In:   demux.InFrontend,
		Out:  demux.OutTSTap,
		Type: demux.Other,
	}
	filter, err := dmx.StreamFilter(&filterParam)
	checkErr(err)
	checkErr(filter.Start())

	file, err := os.Open(dvrpath)
	checkErr(err)

	var pcr PCR

	go pcr.Loop(ts.NewPktStream(file))

	for {
		time.Sleep(5 * time.Second)
		pcr.PrintReport()
	}
}
