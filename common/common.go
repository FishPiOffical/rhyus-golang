package common

type (
	GuLog       byte
	GuSqliteLog byte
	GuFile      byte
	GuFileLock  byte
	GuOs        byte
	GuRand      byte
	GuRuntime   byte
	GuPool      byte
)

var (
	Log      GuLog
	SqlLog   GuSqliteLog
	File     GuFile
	FileLock GuFileLock
	OS       GuOs
	Rand     GuRand
	Runtime  GuRuntime
	Pool     GuPool
)
