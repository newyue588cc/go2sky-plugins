//
// Copyright 2022 SkyAPM org
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

//Package v2 is a HTTP web framework written in
//Go (Golang) plugin which can be used for integration with Gin http server.
package v2

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/SkyAPM/go2sky"
	"github.com/gin-gonic/gin"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

const componentIDGINHttpServer = 5006

type routeInfo struct {
	operationName string
}

type middleware struct {
	routeMap     map[string]map[string]routeInfo
	routeMapOnce sync.Once
}

//Middleware gin middleware return HandlerFunc  with tracing.
func Middleware(engine *gin.Engine, tracer *go2sky.Tracer) gin.HandlerFunc {
	if engine == nil || tracer == nil {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	m := new(middleware)

	return func(c *gin.Context) {
		m.routeMapOnce.Do(func() {
			routes := engine.Routes()
			rm := make(map[string]map[string]routeInfo)
			for _, r := range routes {
				mm := rm[r.Method]
				if mm == nil {
					mm = make(map[string]routeInfo)
					rm[r.Method] = mm
				}
				mm[r.Handler] = routeInfo{
					operationName: fmt.Sprintf("/%s%s", r.Method, r.Path),
				}
			}
			m.routeMap = rm
		})
		var operationName string
		handlerName := c.HandlerName()
		if routeInfo, ok := m.routeMap[c.Request.Method][handlerName]; ok {
			operationName = routeInfo.operationName
		}
		if operationName == "" {
			operationName = c.Request.Method
		}
		span, ctx, err := tracer.CreateEntrySpan(c.Request.Context(), operationName, func(key string) (string, error) {
			return c.Request.Header.Get(key), nil
		})
		if err != nil {
			c.Next()
			return
		}
		span.SetComponent(componentIDGINHttpServer)
		span.Tag(go2sky.TagHTTPMethod, c.Request.Method)
		span.Tag(go2sky.TagURL, c.Request.Host+c.Request.URL.Path)
		span.SetSpanLayer(agentv3.SpanLayer_Http)

		c.Request = c.Request.WithContext(ctx)

		c.Next()

		if len(c.Errors) > 0 {
			span.Error(time.Now(), c.Errors.String())
		}
		span.Tag(go2sky.TagStatusCode, strconv.Itoa(c.Writer.Status()))
		span.End()
	}
}
