package update 

import (
    log "microagent/common/formatlog"
    "microagent/controller"
    "io"
    "os"
)

var Update *UpdateOper

type UpdateOper struct {
    CurVersion string
    UpdateSwitch bool
}

func NewUpdate() *UpdateOper {
    updateOper := UpdateOper{
        CurVersion: "v1.0",
        UpdateSwitch: false,

    }
    return &updateOper
}

func (up *UpdateOper) CheckUpdate() {
	lastVersion, err := controller.Commctrl.GetLatestVersion()
	if err != nil {
		log.Errorf("[Agent] 获取agent最新版本失败, 请检查接口, 错误信息: %s", err.Error())
		return
	}
    
    if lastVersion != up.CurVersion {
        log.Infof("[Agent] 当前最新Agent 版本为 %s, 运行版本为 %s, 需要更新", lastVersion, up.CurVersion)
        err = controller.Commctrl.DownloadAgent()
        if err != nil {
		    log.Errorf("[Agent] 下载agent最新版本失败, 请检查接口, 错误信息: %s", err.Error())
		    return            
        }
        // 下载完成，拷贝文件
        srcFile, err := os.Open("./bin/newagent")
        if err != nil {
            return
        }
        defer srcFile.Close()

        dstFile, err := os.OpenFile("./bin/nowagent", os.O_WRONLY|os.O_CREATE, 0755)
        if err != nil {
            return
        }
        defer dstFile.Close()

        io.Copy(srcFile, dstFile) 
        log.Infoln("[Agent] 当前最新Agent 拷贝完成")
        up.UpdateSwitch = true

    }
    log.Infof("[Agent] 当前最新Agent 版本为 %s, 运行版本为 %s, 无需更新", lastVersion, up.CurVersion)
}
