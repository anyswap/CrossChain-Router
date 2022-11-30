package bridge

import (
	"path/filepath"

	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/fsnotify/fsnotify"
)

// WatchGatewayConfig watch and update gateway config
func WatchGatewayConfig() {
	if params.GatewayConfigFile == "" {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error("fsnotify: new watcher failed", "err", err)
		return
	}

	go startWatcher(watcher)

	file := filepath.Clean(params.GatewayConfigFile)
	dir := filepath.Dir(file)
	err = watcher.Add(dir)
	if err != nil {
		log.Error("fsnotify: add gateway path failed", "err", err)
		return
	}
	log.Infof("fsnotify: start to watch gateway config file %v", file)
}

func startWatcher(watcher *fsnotify.Watcher) {
	defer watcher.Close()

	ops := []fsnotify.Op{
		fsnotify.Write,
	}

	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok { // Channel was closed
				log.Error("fsnotify: channel was closed")
				return
			}
			if filepath.Clean(ev.Name) != filepath.Clean(params.GatewayConfigFile) {
				continue
			}
			log.Trace("fsnotify: watcher event", "file", ev.Name, "op", ev.Op)
			for _, op := range ops {
				if ev.Has(op) {
					err := updateGateway(ev.Name)
					if err == nil {
						log.Info("fsnotify: updateGateway success")
					} else {
						log.Warn("fsnotify: updateGateway failed", "err", err)
					}
					break
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok { // Channel was closed
				log.Error("fsnotify: channel was closed")
				return
			}
			log.Warn("fsnotify: watcher error", "err", err)
		}
	}
}

func updateGateway(fileName string) error {
	gateways, err := params.LoadGatewayConfigs()
	if err != nil {
		return err
	}

	params.GetRouterConfig().GatewayConfigs = gateways
	router.RouterBridges.Range(func(k, v interface{}) bool {
		chainID := k.(string)
		bridge := v.(tokens.IBridge)

		SetGatewayConfig(bridge, chainID)
		return true
	})

	return nil
}
