package fetch

import (
	"fmt"
	"net/http"
	"time"

	"../changelog"
)

// Repo represents a remote slackware software repo
type Repo struct {
	URL string
}

func (r Repo) head(file string) (*http.Response, error) {
	return http.Head(r.URL + "/" + file)
}
func (r Repo) get(file string) (*http.Response, error) {
	return http.Get(r.URL + "/" + file)
}

func (r Repo) NewerChangeLog(than time.Time) (e []changelog.Entry, mtime time.Time, err error) {
	resp, err := r.head("ChangeLog.txt")
	if err != nil {
		return nil, time.Unix(0, 0), err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, time.Unix(0, 0), fmt.Errorf("%d status from %s", resp.StatusCode, resp.Request.URL)
	}

	mtime, err = http.ParseTime(resp.Header.Get("last-modified"))
	if err != nil {
		return nil, time.Unix(0, 0), err
	}

	if mtime.After(than) {
		return r.ChangeLog()
	}
	return nil, time.Unix(0, 0), NotNewer
}

// NotNewer is a status error usage to indicate that the remote file is not newer
var NotNewer = fmt.Errorf("Remote file is not newer than provided time")

// ChangeLog fetches the ChangeLog.txt for this remote Repo, along with the
// last-modified (for comparisons).
func (r Repo) ChangeLog() (e []changelog.Entry, mtime time.Time, err error) {
	resp, err := r.get("ChangeLog.txt")
	if err != nil {
		return nil, time.Unix(0, 0), err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, time.Unix(0, 0), fmt.Errorf("%d status from %s", resp.StatusCode, resp.Request.URL)
	}

	mtime, err = http.ParseTime(resp.Header.Get("last-modified"))
	if err != nil {
		return nil, time.Unix(0, 0), err
	}

	e, err = changelog.Parse(resp.Body)
	if err != nil {
		return nil, mtime, err
	}
	return e, mtime, nil
}
