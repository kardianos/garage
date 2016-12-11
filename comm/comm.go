package comm

type CmdType byte

const (
	CmdRespOK    CmdType = 200
	CmdRespFail          = 500
	CmdReqPing           = 100
	CmdReqClose          = 101
	CmdReqToggle         = 150
)

func Port() int {
	return port
}

func Host() string {
	return host
}

func CA() []byte {
	return []byte(ca)
}

func Cert() []byte {
	return []byte(cert)
}

func Key() []byte {
	return []byte(key)
}
