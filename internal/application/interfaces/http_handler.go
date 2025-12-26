package interfaces

import stdhttp "net/http"

type HTTPHandler interface {
	stdhttp.Handler
}
