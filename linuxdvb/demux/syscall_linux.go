package demux

const (
	_DMX_START           = 0x6f29
	_DMX_STOP            = 0x6f2a
	_DMX_SET_BUFFER_SIZE = 0x6f2d
	_DMX_SET_FILTER      = 0x403c6f2b
	_DMX_SET_PES_FILTER  = 0x40146f2c
	_DMX_GET_PES_PIDS    = 0x800a6f2f
	_DMX_GET_CAPS        = 0x80086f30
	_DMX_SET_SOURCE      = 0x40046f31
	_DMX_GET_STC         = 0xc0106f32
	_DMX_ADD_PID         = 0x40026f33
	_DMX_REMOVE_PID      = 0x40026f34
)
