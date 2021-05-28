package redis

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gomodule/redigo/redis"
)

// Pool ...
type Pool struct {
	pool    *redis.Pool
	scripts map[string]*redis.Script
}

func (p *Pool) loadScript(script string, loadScript func(string, string)) error {
	if script == "" {
		return nil
	}

	return filepath.Walk(script,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			var (
				fileName   = info.Name()
				fileprefix string
				count      string
			)

			if !strings.HasSuffix(fileName, LuaFileSuffix) {
				return nil
			}

			fileprefix = strings.TrimSuffix(fileName, LuaFileSuffix)
			for i := len(fileprefix) - 1; i >= 0; i-- {
				if fileprefix[i] != '_' {
					continue
				}

				count = fileprefix[i+1:]
				c, err := strconv.ParseInt(count, 10, 64)
				if err != nil {
					return nil
				}

				f, err := ioutil.ReadFile(path)
				if err != nil {
					return err
				}

				s := redis.NewScript(int(c), string(f))
				err = s.Load(p.pool.Get())
				if err != nil {
					return err
				}
				p.scripts[fileprefix] = s
				if loadScript != nil {
					loadScript(fileprefix, s.Hash())
				}
				return nil
			}
			return nil
		})
}
