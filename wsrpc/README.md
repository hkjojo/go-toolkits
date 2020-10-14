# wsrpc examples
## server
```go
func rpcInit() {
    var (
		router = gin.New()
		rpcSrv = rpc.NewServer()
	)

	router.Use(cors.New(cors.Config{
		AllowMethods: []string{"OPTIONS", "POST", "GET"},
		AllowHeaders: []string{"Origin", "X-Requested-With",
			"Content-Type", "Accept"},
		AllowCredentials: true,
		AllowOriginFunc: func(origin string) bool {
			return true
		},
	}))

    router.GET("/rpc", func(c *gin.Context) {
        serveRPC(c, rpcSrv)
    })

	rpcSrv.RegisterName("User", &User{})
	rpcSrv.OnWarp(WrapHandler)
	rpcSrv.OnMissingMethod(backendHandler)
}

func serveRPC(c *gin.Context, rpcSrv *rpc.Server) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	ws, err := upgrader.Upgrade(c.Writer, c.Request, c.Header)
	if err != nil {
		log.WithError(err).Warn("Jsonrpc upgrade failed.")
		return
	}

	rpcSrv.OnConnect(c.Request, ws, func(conn *rpc.Conn) {
		// init connect handler
	})
}

type User struct{}

func (u *User) Login(conn *rpc.Conn, req *pbu.LoginReq, rsp *pbu.LoginRsp) error {
    // login handler
	return nil
}

func backendHandler (conn *rpc.Conn, method string, args json.RawMessage) (rsp interface{},err error) {
    // if missing method then run this handler
    return
}
```

## client body
```json
{
    "id": 1,
    "jsonrpc": "2.0",
    "method": "User.Login",
    "params": {}
}
```