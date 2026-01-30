// Package protocol provides protocol constants and utilities
package protocol

// Request command constants - matches Scouter RequestCmd
const (
	// Login/Session commands
	CMD_LOGIN                = "LOGIN"
	CMD_CHECK_SESSION        = "CHECK_SESSION"
	CMD_CHECK_SESSION_ID     = "CHECK_SESSION_ID"

	// Object commands
	CMD_OBJECT_LIST_REAL_TIME     = "OBJECT_LIST_REAL_TIME"
	CMD_OBJECT_LIST_ALL           = "OBJECT_LIST_ALL"
	CMD_OBJECT_REMOVE             = "OBJECT_REMOVE"
	CMD_OBJECT_RESET_CACHE        = "OBJECT_RESET_CACHE"
	CMD_OBJECT_CALL_AGENT         = "OBJECT_CALL_AGENT"

	// Counter commands (real-time)
	CMD_COUNTER_REAL_TIME         = "COUNTER_REAL_TIME"
	CMD_COUNTER_REAL_TIME_ALL     = "COUNTER_REAL_TIME_ALL"
	CMD_COUNTER_REAL_TIME_MULTI   = "COUNTER_REAL_TIME_MULTI"
	CMD_COUNTER_REAL_TIME_TOTAL   = "COUNTER_REAL_TIME_TOTAL"
	CMD_COUNTER_REAL_TIME_TOT_ALL = "COUNTER_REAL_TIME_TOT_ALL"
	CMD_COUNTER_REAL_TIME_GROUP   = "COUNTER_REAL_TIME_GROUP"

	// Counter commands (past data)
	CMD_COUNTER_PAST_TIME         = "COUNTER_PAST_TIME"
	CMD_COUNTER_PAST_TIME_ALL     = "COUNTER_PAST_TIME_ALL"
	CMD_COUNTER_PAST_TIME_MULTI   = "COUNTER_PAST_TIME_MULTI"
	CMD_COUNTER_PAST_TIME_TOTAL   = "COUNTER_PAST_TIME_TOTAL"

	// Counter commands (range)
	CMD_COUNTER_LOAD_TIME_GROUP   = "COUNTER_LOAD_TIME_GROUP"
	CMD_COUNTER_LOAD_DATE         = "COUNTER_LOAD_DATE"
	CMD_COUNTER_LOAD_DATE_TOTAL   = "COUNTER_LOAD_DATE_TOTAL"
	CMD_COUNTER_LOAD_TIME         = "COUNTER_LOAD_TIME"
	CMD_COUNTER_LOAD_TIME_TOTAL   = "COUNTER_LOAD_TIME_TOTAL"

	// Active service commands
	CMD_ACTIVE_SERVICE_LIST       = "ACTIVE_SERVICE_LIST"
	CMD_ACTIVE_THREAD             = "ACTIVE_THREAD"
	CMD_ACTIVE_THREAD_LIST        = "ACTIVE_THREAD_LIST"
	CMD_ACTIVE_SPEED              = "ACTIVE_SPEED"
	CMD_ACTIVESPEED_GROUP_REAL_TIME = "ACTIVESPEED_GROUP_REAL_TIME"
	CMD_THREAD_DUMP               = "THREAD_DUMP"
	CMD_THREAD_STOP               = "THREAD_STOP"

	// XLog commands
	CMD_XLOG_READ_BY_TXID         = "XLOG_READ_BY_TXID"
	CMD_XLOG_READ_BY_GXID         = "XLOG_READ_BY_GXID"
	CMD_XLOG_LOAD_BY_TXID         = "XLOG_LOAD_BY_TXID"
	CMD_XLOG_REAL_TIME            = "XLOG_REAL_TIME"
	CMD_XLOG_PAST_TIME            = "XLOG_PAST_TIME"
	CMD_XLOG_LOAD_TIME_RANGE      = "XLOG_LOAD_TIME_RANGE"
	CMD_XLOG_LOAD_DATE_RANGE      = "XLOG_LOAD_DATE_RANGE"
	CMD_TRANX_REAL_TIME_GROUP     = "TRANX_REAL_TIME_GROUP"

	// Profile commands
	CMD_XLOG_PROFILE              = "XLOG_PROFILE"
	CMD_XLOG_PROFILE_FULL         = "XLOG_PROFILE_FULL"

	// Text commands
	CMD_GET_TEXT                  = "GET_TEXT"
	CMD_GET_TEXT_PACK             = "GET_TEXT_PACK"
	CMD_GET_TEXT_100              = "GET_TEXT_100"

	// Alert commands
	CMD_ALERT_REAL_TIME           = "ALERT_REAL_TIME"
	CMD_ALERT_LOAD_TIME           = "ALERT_LOAD_TIME"
	CMD_ALERT_LOAD_TIME_RANGE     = "ALERT_LOAD_TIME_RANGE"

	// Summary commands
	CMD_SUMMARY_APM_SERVICE       = "SUMMARY_APM_SERVICE"
	CMD_SUMMARY_APM_SQL           = "SUMMARY_APM_SQL"
	CMD_SUMMARY_APM_APICALL       = "SUMMARY_APM_APICALL"
	CMD_SUMMARY_APM_IP            = "SUMMARY_APM_IP"
	CMD_SUMMARY_APM_USER          = "SUMMARY_APM_USER"
	CMD_SUMMARY_APM_ERROR         = "SUMMARY_APM_ERROR"

	// Host commands
	CMD_HOST_DISK_USAGE           = "HOST_DISK_USAGE"
	CMD_HOST_NET_STAT             = "HOST_NET_STAT"
	CMD_HOST_TOP                  = "HOST_TOP"
	CMD_HOST_PROCESS              = "HOST_PROCESS"
	CMD_HOST_WHO                  = "HOST_WHO"
	CMD_HOST_MEMINFO              = "HOST_MEMINFO"

	// Server info commands
	CMD_SERVER_INFO               = "SERVER_INFO"
	CMD_SERVER_COUNTER_XML        = "SERVER_COUNTER_XML"
	CMD_SERVER_TIMEZONE           = "SERVER_TIMEZONE"
	CMD_SERVER_VERSION            = "SERVER_VERSION"
	CMD_SERVER_TIME               = "SERVER_TIME"
	CMD_SERVER_STATUS             = "SERVER_STATUS"

	// Counter type commands
	CMD_GET_COUNTER_LIST          = "GET_COUNTER_LIST"
	CMD_GET_COUNTER_NAMES         = "GET_COUNTER_NAMES"
)

// Parameter key constants - common request parameter names
const (
	ParamObjHash      = "objHash"
	ParamObjHashes    = "objHashs"
	ParamObjName      = "objName"
	ParamObjType      = "objType"
	ParamCounter      = "counter"
	ParamCounters     = "counters"
	ParamFromTime     = "stime"
	ParamToTime       = "etime"
	ParamDate         = "date"
	ParamServerId     = "serverId"
	ParamTxID         = "txid"
	ParamGxID         = "gxid"
	ParamTextType     = "type"
	ParamHashValue    = "hash"
	ParamMax          = "max"
	ParamLogin        = "id"
	ParamPassword     = "pass"
	ParamSessionID    = "scouter_session"
)

// Response key constants - common response field names
const (
	ResponseSessionID = "scouter_session"
	ResponseVersion   = "version"
	ResponseSuccess   = "success"
	ResponseMessage   = "message"
	ResponseTime      = "time"
)
