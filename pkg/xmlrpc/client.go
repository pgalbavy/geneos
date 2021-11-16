package xmlrpc // import "wonderland.org/geneos/pkg/xmlrpc"

import (
	"crypto/tls"
	"net/http"
)

/*
The Client struct carries the http Client and the url down to successive layers
*/
type Client struct {
	http.Client
	url string
}

/*
ToString is a convenience function to render the structure to some sort of readable format.

Successive layers implement their own and add more data
*/
func (c Client) ToString() string {
	return c.URL()
}

/*
IsValid returns a boolean based on the semantics of the layer it's call against.

At the top Client level it checks if the Gateway is connected to the Netprobe, but
further levels will test if the appropriate objects exist in the Netprobe
*/
func (c Client) IsValid() bool {
	res, err := c.gatewayConnected()
	if err != nil {
		ErrorLogger.Print(err)
		return false
	}
	return res
}

/*
URL returns the configured root URL of the XMLRPC endpoint
*/
func (c Client) URL() string {
	return c.url
}

/*
SetURL takes a preformatted URL for the client.

The normal format is http[s]://host:port/xmlrpc
*/
func (c *Client) SetURL(url string) {
	c.url = url
}

func (c *Client) AllowUnverifiedCertificates() {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	c.Client = http.Client{Transport: tr}
}

/*
Sampler creates and returns a new Sampler struct from the lower level.

XXX At the moment there is no error checking or validation
*/
func (c Client) NewSampler(entityName string, samplerName string) (sampler Sampler, err error) {
	sampler = Sampler{Client: c, entityName: entityName, samplerName: samplerName}
	return
}
