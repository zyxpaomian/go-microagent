package update 

import (
    log "microagent/common/formatlog"
    "microagent/controller"
    cfg "microagent/common/configparse"
    "os"
)

var Update *UpdateOper

type UpdateOper struct {
    CurVersion string
    NowVersion  string
    NowVersionBak string
    NewVersion string
    UpdateSwitch bool
}

func NewUpdate() *UpdateOper {
    updateOper := UpdateOper{
        CurVersion: "v1.0",
        NowVersion: cfg.GlobalConf.GetStr("package", "nowagent"),
        NowVersionBak: cfg.GlobalConf.GetStr("package", "nowagentbak"),
        NewVersion: cfg.GlobalConf.GetStr("package", "newagent"),
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
        // 删除历史版本客户端
        if _, err := os.Stat(up.NewVersion); err == nil {
            os.RemoveAll(up.NewVersion)
            log.Infoln("[Agent] 删除历史版本的Agent完成, 等待Agent 重启")
        }

        // 下载历史版本客户端
        err = controller.Commctrl.DownloadAgent()
        if err != nil {
		    log.Errorf("[Agent] 下载agent最新版本失败, 请检查接口, 错误信息: %s", err.Error())
		    return            
        }

        // 移动当前版本客户端
        err = os.Rename(up.NowVersion, up.NowVersionBak)
        if err != nil {
            log.Errorf("[Agent] 移动当前老版本Agent失败, 客户端版本:%v", err.Error())
            return   
        }
        log.Infoln("[Agent] 移动当前老版本Agent完成, 等待Agent 重启")

        // 移动下载到当前
        err = os.Rename(up.NewVersion, up.NowVersion)
        if err != nil {
            log.Errorf("[Agent] 移动下载版本Agent到最新失败, 客户端版本:%v", err.Error())
            return   
        }
        log.Infoln("[Agent] 移动下载版本Agent到最新完成, 等待Agent 重启")        
        up.UpdateSwitch = true

    }
    log.Infof("[Agent] 当前最新Agent 版本为 %s, 运行版本为 %s, 无需更新", lastVersion, up.CurVersion)
}
