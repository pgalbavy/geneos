package xmlrpc // import "wonderland.org/geneos/xmlrpc"

import (
	_ "fmt"
	"time"
)

func (s Sampler) WriteMessage(streamname string, message string) (err error) {
	return s.addMessageStream(s.EntityName(), s.SamplerName(), streamname, message)
}

func (s Sampler) SignOn(streamname string, heartbeat time.Duration) error {
	return s.signOnStream(s.EntityName(), s.SamplerName(), streamname, int(heartbeat.Seconds()))
}

func (s Sampler) SignOff(streamname string) error {
	return s.signOffStream(s.EntityName(), s.SamplerName(), streamname)
}

func (s Sampler) Heartbeat(streamname string) error {
	return s.heartbeatStream(s.EntityName(), s.SamplerName(), streamname)
}
