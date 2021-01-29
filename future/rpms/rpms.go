package rpms 

import (
    "os/exec"
    "strings"
    "bytes"
    "time"
    log "microagent/common/formatlog"
)

var Rpm *RpmOper

type RpmOper struct {
    ColTime string
    RpmList []string
}

func NewRpm() *RpmOper {
    rpmOper := RpmOper{
        ColTime: time.Now().Format("2006/1/2 15:04:05"),
    }
    return &rpmOper
}

func (rp *RpmOper) GetAllRpms() {
    cmdDetail := "rpm -qa"
    cmd := exec.Command("bash", "-c", cmdDetail)
    
	var stdOut, stdErr bytes.Buffer
    cmd.Stdout = &stdOut
    cmd.Stderr = &stdErr

    err := cmd.Run()
    if err != nil {
        log.Errorf("[Rpms] 收集所有RPM 失败, 错误原因: %s", err.Error())
        return
    }   
    outStr, errStr := string(stdOut.Bytes()), string(stdErr.Bytes())
    if len(errStr) != 0 {
        log.Errorf("[Rpms] 收集所有RPM 失败, 错误原因: %s", err.Error())
    } else {
        rpmList := strings.Split(outStr, "\n")
        for i, v := range rpmList {
            if len(v) == 0 {
                rpmList = append(rpmList[:i], rpmList[i+1:]...)
            }
        }
        rp.RpmList = rpmList 
    }
}
