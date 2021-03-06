package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli"
	"github.com/vbatts/sl-feeds/changelog"
	"github.com/vbatts/sl-feeds/fetch"
)

func main() {
	config := Config{}

	app := cli.NewApp()
	app.Name = "sl-feeds"
	app.Usage = "Transform slackware ChangeLog.txt into RSS feeds"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Usage: "Load configuration from `FILE`",
		},
		cli.StringFlag{
			Name:  "dest, d",
			Usage: "Output RSS files to `DIR`",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Less output",
		},
		cli.BoolFlag{
			Name:  "insecure",
			Usage: "do not validate server certificate",
		},
		cli.StringFlag{
			Name:  "ca",
			Usage: "additional CA cert to use",
		},
		cli.BoolFlag{
			Name:  "sample-config",
			Usage: "Output sample config file to stdout",
		},
	}

	// This is the main/default application
	app.Action = func(c *cli.Context) error {
		rootCAs, _ := x509.SystemCertPool()
		if c.String("ca") != "" {
			if rootCAs == nil {
				rootCAs = x509.NewCertPool()
			}
			// Read in the cert file
			certs, err := ioutil.ReadFile(c.String("ca"))
			if err != nil {
				log.Fatalf("Failed to append %q to RootCAs: %v", c.String("ca"), err)
			}

			// Append our cert to the system pool
			if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
				log.Println("No certs appended, using system certs only")
			}
		}
		if c.Bool("insecure") {
			config := &tls.Config{
				InsecureSkipVerify: true,
				RootCAs:            rootCAs,
			}
			http.DefaultTransport = &http.Transport{TLSClientConfig: config}
		}
		if c.Bool("sample-config") {
			c := Config{
				Dest:  "$HOME/public_html/feeds/",
				Quiet: false,
				Mirrors: []Mirror{
					Mirror{
						URL: "http://slackware.osuosl.org/",
						Releases: []string{
							"slackware-14.0",
							"slackware-14.1",
							"slackware-14.2",
							"slackware-current",
							"slackware64-14.0",
							"slackware64-14.1",
							"slackware64-14.2",
							"slackware64-current",
						},
					},
					Mirror{
						URL: "http://ftp.arm.slackware.com/slackwarearm/",
						Releases: []string{
							"slackwarearm-14.2",
							"slackwarearm-current",
						},
					},
					Mirror{
						URL:    "http://alphageek.noip.me/mirrors/alphageek/",
						Prefix: "alphageek-",
						Releases: []string{
							"slackware64-14.2",
						},
					},
				},
			}
			toml.NewEncoder(os.Stdout).Encode(c)
			return nil
		}

		dest := os.ExpandEnv(config.Dest)
		if !c.Bool("quiet") {
			fmt.Printf("Writing to: %q\n", dest)
		}
		/*
			for each mirror in Mirrors
				if there is not a $release.RSS file, then fetch the whole ChangeLog
				if there is a $release.RSS file, then stat the file and only fetch remote if it is newer than the local RSS file
				if the remote returns any error (404, 503, etc) then print a warning but continue
		*/
		for _, mirror := range config.Mirrors {
			for _, release := range mirror.Releases {
				repo := fetch.Repo{
					URL:     mirror.URL,
					Release: release,
				}

				if !c.Bool("quiet") {
					log.Printf("processing %q", repo.URL+"/"+repo.Release)
				}

				stat, err := os.Stat(filepath.Join(dest, mirror.Prefix+release+".rss"))
				if err != nil && !os.IsNotExist(err) {
					log.Println(release, err)
					continue
				}
				var (
					entries []changelog.Entry
					mtime   time.Time
				)
				if os.IsNotExist(err) {
					entries, mtime, err = repo.ChangeLog()
					if err != nil {
						log.Println(release, err)
						continue
					}
				} else {
					// compare times
					entries, mtime, err = repo.NewerChangeLog(stat.ModTime())
					if err != nil {
						if !(err == fetch.ErrNotNewer && c.Bool("quiet")) {
							log.Println(release, err)
						}
						continue
					}
				}

				// write out the rss and chtime it to be mtime
				feeds, err := changelog.ToFeed(repo.URL+"/"+release, entries)
				if err != nil {
					log.Println(release, err)
					continue
				}
				feeds.Title = fmt.Sprintf("ChangeLog.txt for %s%s", mirror.Prefix, release)
				fh, err := os.Create(filepath.Join(dest, mirror.Prefix+release+".rss"))
				if err != nil {
					log.Println(release, err)
					continue
				}
				if err := feeds.WriteRss(fh); err != nil {
					log.Println(release, err)
					fh.Close()
					continue
				}
				fh.Close()
				err = os.Chtimes(filepath.Join(dest, mirror.Prefix+release+".rss"), mtime, mtime)
				if err != nil {
					log.Println(release, err)
					continue
				}
			}
		}
		return nil
	}

	app.Before = func(c *cli.Context) error {
		if c.String("config") == "" {
			return nil
		}

		data, err := ioutil.ReadFile(c.String("config"))
		if err != nil {
			return err
		}
		if _, err := toml.Decode(string(data), &config); err != nil {
			return err
		}
		if c.String("dest") != "" {
			config.Dest = c.String("dest")
		}
		return nil
	}

	app.Run(os.Args)
}

// Config is read in to point to where RSS are written to, and the Mirrors to
// be fetched from
type Config struct {
	Quiet   bool
	Dest    string
	Mirrors []Mirror
}

// Mirror is where the release/ChangeLog.txt will be fetched from
type Mirror struct {
	URL      string
	Releases []string
	Prefix   string
}
