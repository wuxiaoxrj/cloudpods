// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"golang.org/x/net/http/httpproxy"

	"yunion.io/x/structarg"

	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/multicloud/openstack"
	_ "yunion.io/x/onecloud/pkg/multicloud/openstack/shell"
	"yunion.io/x/onecloud/pkg/util/shellutils"
)

type BaseOptions struct {
	Debug         bool   `help:"debug mode"`
	Help          bool   `help:"Show help"`
	AuthURL       string `help:"Auth URL" default:"$OPENSTACK_AUTH_URL" metavar:"OPENSTACK_AUTH_URL"`
	Username      string `help:"Username" default:"$OPENSTACK_USERNAME" metavar:"OPENSTACK_USERNAME"`
	Password      string `help:"Password" default:"$OPENSTACK_PASSWORD" metavar:"OPENSTACK_PASSWORD"`
	Project       string `help:"Project" default:"$OPENSTACK_PROJECT" metavar:"OPENSTACK_PROJECT"`
	EndpointType  string `help:"Project" default:"$OPENSTACK_ENDPOINT_TYPE|internal" metavar:"OPENSTACK_ENDPOINT_TYPE"`
	DomainName    string `help:"Domain of user" default:"$OPENSTACK_DOMAIN_NAME|Default" metavar:"OPENSTACK_DOMAIN_NAME"`
	ProjectDomain string `help:"Domain of project" default:"$OPENSTACK_PROJECT_DOMAIN|Default" metavar:"OPENSTACK_PROJECT_DOMAIN"`
	RegionID      string `help:"RegionId" default:"$OPENSTACK_REGION_ID" metavar:"OPENSTACK_REGION_ID"`
	SUBCOMMAND    string `help:"openstackcli subcommand" subcommand:"true"`
}

func getSubcommandParser() (*structarg.ArgumentParser, error) {
	parse, e := structarg.NewArgumentParser(&BaseOptions{},
		"openstackcli",
		"Command-line interface to openstack API.",
		`See "openstackcli help COMMAND" for help on a specific command.`)

	if e != nil {
		return nil, e
	}

	subcmd := parse.GetSubcommand()
	if subcmd == nil {
		return nil, fmt.Errorf("No subcommand argument.")
	}
	type HelpOptions struct {
		SUBCOMMAND string `help:"sub-command name"`
	}
	shellutils.R(&HelpOptions{}, "help", "Show help of a subcommand", func(args *HelpOptions) error {
		helpstr, e := subcmd.SubHelpString(args.SUBCOMMAND)
		if e != nil {
			return e
		} else {
			fmt.Print(helpstr)
			return nil
		}
	})
	for _, v := range shellutils.CommandTable {
		_, e := subcmd.AddSubParser(v.Options, v.Command, v.Desc, v.Callback)
		if e != nil {
			return nil, e
		}
	}
	return parse, nil
}

func showErrorAndExit(e error) {
	fmt.Fprintf(os.Stderr, "%s", e)
	fmt.Fprintln(os.Stderr)
	os.Exit(1)
}

func newClient(options *BaseOptions) (*openstack.SRegion, error) {
	if len(options.AuthURL) == 0 {
		return nil, fmt.Errorf("Missing AuthURL")
	}

	if len(options.Username) == 0 {
		return nil, fmt.Errorf("Missing Username")
	}

	if len(options.Password) == 0 {
		return nil, fmt.Errorf("Missing Password")
	}

	cfg := &httpproxy.Config{
		HTTPProxy:  os.Getenv("HTTP_PROXY"),
		HTTPSProxy: os.Getenv("HTTPS_PROXY"),
		NoProxy:    os.Getenv("NO_PROXY"),
	}
	cfgProxyFunc := cfg.ProxyFunc()
	proxyFunc := func(req *http.Request) (*url.URL, error) {
		return cfgProxyFunc(req.URL)
	}

	cli, err := openstack.NewOpenStackClient(
		openstack.NewOpenstackClientConfig(
			options.AuthURL,
			options.Username,
			options.Password,
			options.Project,
			options.ProjectDomain,
		).
			EndpointType(options.EndpointType).
			DomainName(options.DomainName).
			Debug(options.Debug).
			CloudproviderConfig(
				cloudprovider.ProviderConfig{
					ProxyFunc: proxyFunc,
				},
			),
	)
	if err != nil {
		return nil, err
	}
	region := cli.GetRegion(options.RegionID)
	if region == nil {
		return nil, fmt.Errorf("No such region %s", options.RegionID)
	}
	return region, nil
}

func main() {
	parser, e := getSubcommandParser()
	if e != nil {
		showErrorAndExit(e)
	}
	e = parser.ParseArgs(os.Args[1:], false)
	options := parser.Options().(*BaseOptions)

	if options.Help {
		fmt.Print(parser.HelpString())
	} else {
		subcmd := parser.GetSubcommand()
		subparser := subcmd.GetSubParser()
		if e != nil {
			if subparser != nil {
				fmt.Print(subparser.Usage())
			} else {
				fmt.Print(parser.Usage())
			}
			showErrorAndExit(e)
		} else {
			suboptions := subparser.Options()
			if options.SUBCOMMAND == "help" {
				e = subcmd.Invoke(suboptions)
			} else {
				var region *openstack.SRegion
				if len(options.RegionID) == 0 {
					options.RegionID = openstack.OPENSTACK_DEFAULT_REGION
				}
				region, e = newClient(options)
				if e != nil {
					showErrorAndExit(e)
				}
				e = subcmd.Invoke(region, suboptions)
			}
			if e != nil {
				showErrorAndExit(e)
			}
		}
	}
}
