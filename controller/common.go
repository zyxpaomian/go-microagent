package controller
import (
    cfg "microagent/common/configparse"
	log "microagent/common/formatlog"
    ae "microagent/common/error"
    "microagent/common"
    "strings"
    "io/ioutil"
    "net/http"
    "fmt"
    "os"
    "io"
)

var Commctrl *CommCtrl

type CommCtrl struct {
}

func init() {
	Commctrl = &CommCtrl{}
}


func (c *CommCtrl) ElectServer() (string, error) {
    svrAddrsStr := cfg.GlobalConf.GetStr("common", "svraddrs")
    svrAddrList := strings.Split(svrAddrsStr, ",")
    svrAgentNumMap := make(map[string]int)

    for _, svrAddr := range(svrAddrList) {
        svrAgentNumMap[svrAddr] = 0

        // 拼接一下获取每个server段当前Agent 数量的接口
        svrIp := strings.Split(svrAddr, ":")[0]
        getSvrApi := "http://" + svrIp + ":8080/" + cfg.GlobalConf.GetStr("api", "apigetagentsnum")
    	
        // 调接口
        hc := &http.Client{}
	    req, err := http.NewRequest("GET", getSvrApi, nil)
	    if err != nil {
            log.Errorf("[Agent] 获取svr: %s 连接数失败，错误信息: %s", svrAddr, err.Error())
		    return "", ae.New(fmt.Sprintf("[Agent] 获取服务器最小链接失败, 错误信息: %s", err.Error()))
	    }
	    resp, err := hc.Do(req)
	    if err != nil {
            log.Errorf("[Agent] 获取svr: %s 连接数失败，错误信息: %s", svrAddr, err.Error())            
		    return "", ae.New(fmt.Sprintf("[Agent] 获取服务器最小链接失败, 错误信息: %s", err.Error()))
	    }

	    defer resp.Body.Close()
	    respContent, _ := ioutil.ReadAll(resp.Body)
	    Content := string(respContent)

	    if resp.StatusCode != 200 {
            log.Errorf("[Agent] 获取svr: %s 连接数失败，错误信息: %s", svrAddr, err.Error())
		    return "", ae.New(fmt.Sprintf("[Agent] 获取服务器最小链接失败, 错误信息: %s", err.Error()))
	    }

        type MsgResult struct {
            Agentsnum   int `json:"agentsnum\"`
        }

        msgresult := MsgResult{}
	    if err := common.ParseJsonStr(Content, &msgresult); err != nil {
            log.Errorf("[Agent] 获取svr: %s 连接数失败，解析模板JSON失败, 错误信息: %s", svrAddr, err.Error())
		    return "", ae.New(fmt.Sprintf("[Agent] 获取服务器最小链接失败, 错误信息: %s", err.Error()))
	    }
        svrAgentNumMap[svrAddr] = msgresult.Agentsnum
	    log.Infof("[Agent]  svr: %s 连接数 : %d", svrAddr, msgresult.Agentsnum)
    }

    // 排序，找个最小链接的服务器
    for i := 0; i< len(svrAddrList); i++ {
        for j := i + 1; j < len(svrAddrList); j++ {
            if svrAgentNumMap[svrAddrList[j]] < svrAgentNumMap[svrAddrList[i]] {
                svrAddrList[i], svrAddrList[j] = svrAddrList[j], svrAddrList[i]
            }
        }
    }
    log.Infof("[Agent] 当前客户端连接服务端 %v", svrAddrList)
    return svrAddrList[0], nil
}

func (c *CommCtrl) GetLatestVersion() (string, error) {
    svrVipStr := cfg.GlobalConf.GetStr("common", "svrvip")
    getVersionApi := "http://" + svrVipStr + ":8080/" + cfg.GlobalConf.GetStr("api", "apigetagentlastestversion")
    	
    // 调接口
    hc := &http.Client{}
	req, err := http.NewRequest("GET", getVersionApi, nil)
	if err != nil {
        log.Errorf("[Agent] 获取agent最新版本失败, 错误信息: %s", err.Error())
		return "", ae.New(fmt.Sprintf("[Agent] 获取agent最新版本失败, 错误信息: %s", err.Error()))
	}
	resp, err := hc.Do(req)
	if err != nil {
        log.Errorf("[Agent] 获取agent最新版本失败, 错误信息: %s", err.Error())            
		return "", ae.New(fmt.Sprintf("[Agent] 获取agent最新版本失败, 错误信息: %s", err.Error()))
	}

	defer resp.Body.Close()
	respContent, _ := ioutil.ReadAll(resp.Body)
	Content := string(respContent)

	if resp.StatusCode != 200 {
        log.Errorf("[Agent] 获取agent最新版本失败, 错误信息: %s", err.Error())
		return "", ae.New(fmt.Sprintf("[Agent] 获取agent最新版本失败, 错误信息: %s", err.Error()))
	}

    type MsgResult struct {
        Lastestversion   string `json:"lastestversion\"`
    }

    msgresult := MsgResult{}
	if err := common.ParseJsonStr(Content, &msgresult); err != nil {
        log.Errorf("[Agent] 获取agent最新版本失败，解析模板JSON失败, 错误信息: %s", err.Error())
		return "", ae.New(fmt.Sprintf("[Agent] 获取agent最新版本失败, 错误信息: %s", err.Error()))
	}

    return msgresult.Lastestversion, nil
}

func (c *CommCtrl) DownloadAgent() (error) {
    svrVipStr := cfg.GlobalConf.GetStr("common", "svrvip")
    downloadApi := "http://" + svrVipStr + ":8080/" + cfg.GlobalConf.GetStr("api", "apidownloadagent")    
    
    // 调接口
    res, err := http.Get(downloadApi)
    if (res != nil) {
    	defer res.Body.Close()
    }
    if err != nil {
        log.Errorf("[Agent] 下载agent最新版本失败，解析模板JSON失败, 错误信息: %s", err.Error())
        return err
    }
    
    downloadFile, err := os.Create(cfg.GlobalConf.GetStr("package", "newagent"))
    if (downloadFile != nil) {
    	defer downloadFile.Close()
    }
    if err != nil {
        log.Errorf("[Agent] 下载agent最新版本失败，解析模板JSON失败, 错误信息: %s", err.Error())
        return err
    }
    
    io.Copy(downloadFile, res.Body) 
    log.Infoln("[Agent] 下载agent最新版本完成，等待更新")
    return nil
}
