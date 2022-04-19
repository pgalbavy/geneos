/*
Copyright © 2022 Peter Galbavy <peter@wonderland.org>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"crypto/sha1"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/instance"
)

// tlsLsCmd represents the tlsLs command
var tlsLsCmd = &cobra.Command{
	Use:   "tlsLs",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("tlsLs called")
	},
}

func init() {
	tlsCmd.AddCommand(tlsLsCmd)

	tlsLsCmd.Flags().BoolVarP(&tlsCmdAll, "all", "a", false, "Show all certs, including global and signing certs")
	tlsLsCmd.Flags().BoolVarP(&tlsCmdJSON, "json", "j", false, "Output JSON")
	tlsLsCmd.Flags().BoolVarP(&tlsCmdLong, "long", "l", false, "Long output")
	tlsLsCmd.Flags().BoolVarP(&tlsCmdIndent, "indent", "i", false, "Indent / pretty print JSON")
	tlsLsCmd.Flags().BoolVarP(&tlsCmdCSV, "csv", "c", false, "Output CSV")
}

var tlsCmdAll, tlsCmdCSV, tlsCmdJSON, tlsCmdIndent, tlsCmdLong bool

type lsCertType struct {
	Type       string
	Name       string
	Location   host.Name
	Remaining  time.Duration
	Expires    time.Time
	CommonName string
}

type lsCertLongType struct {
	Type        string
	Name        string
	Location    host.Name
	Remaining   time.Duration
	Expires     time.Time
	CommonName  string
	Issuer      string
	SubAltNames []string
	IPs         []net.IP
	Signature   string
}

func lsInstanceCert(c geneos.Instance, params []string) (err error) {
	cert, err := instance.ReadCert(c)
	if err == os.ErrNotExist {
		// this is OK - instance.ReadCert() reports no configured cert this way
		return nil
	}
	if err != nil {
		return
	}
	expires := cert.NotAfter
	fmt.Fprintf(lsTabWriter, "%s\t%s\t%s\t%.f\t%q\t%q\t", c.Type(), c.Name(), c.Location(), time.Until(expires).Seconds(), expires, cert.Subject.CommonName)

	if tlsCmdLong {
		fmt.Fprintf(lsTabWriter, "%q\t", cert.Issuer.CommonName)
		if len(cert.DNSNames) > 0 {
			fmt.Fprintf(lsTabWriter, "%v", cert.DNSNames)
		}
		fmt.Fprintf(lsTabWriter, "\t")
		if len(cert.IPAddresses) > 0 {
			fmt.Fprintf(lsTabWriter, "%v", cert.IPAddresses)
		}
		fmt.Fprintf(lsTabWriter, "\t%X", sha1.Sum(cert.Raw))
	}
	fmt.Fprint(lsTabWriter, "\n")
	return
}

func lsInstanceCertCSV(c geneos.Instance, params []string) (err error) {
	cert, err := instance.ReadCert(c)
	if err == os.ErrNotExist {
		// this is OK
		return nil
	}
	if err != nil {
		return
	}
	expires := cert.NotAfter
	until := fmt.Sprintf("%0f", time.Until(expires).Seconds())
	cols := []string{c.Type().String(), c.Name(), string(c.Location()), until, expires.String(), cert.Subject.CommonName}
	if tlsCmdLong {
		cols = append(cols, cert.Issuer.CommonName)
		cols = append(cols, fmt.Sprintf("%v", cert.DNSNames))
		cols = append(cols, fmt.Sprintf("%v", cert.IPAddresses))
		cols = append(cols, fmt.Sprintf("%X", sha1.Sum(cert.Raw)))
	}

	csvWriter.Write(cols)
	return
}

func lsInstanceCertJSON(c geneos.Instance, params []string) (err error) {
	cert, err := instance.ReadCert(c)
	if err == os.ErrNotExist {
		// this is OK
		return nil
	}
	if err != nil {
		return
	}
	if tlsCmdLong {
		jsonEncoder.Encode(lsCertLongType{c.Type().String(), c.Name(), c.Location(), time.Duration(time.Until(cert.NotAfter).Seconds()),
			cert.NotAfter, cert.Subject.CommonName, cert.Issuer.CommonName, cert.DNSNames, cert.IPAddresses, fmt.Sprintf("%X", sha1.Sum(cert.Raw))})
	} else {
		jsonEncoder.Encode(lsCertType{c.Type().String(), c.Name(), c.Location(), time.Duration(time.Until(cert.NotAfter).Seconds()),
			cert.NotAfter, cert.Subject.CommonName})
	}
	return
}
