package plugin

import "github.com/Kong/go-pdk"

type Config interface {
	Access(kong *pdk.PDK)
}
