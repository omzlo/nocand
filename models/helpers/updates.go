package helpers

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func CheckForUpdates(url string) ([]string, error) {

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Download request returned http status code '%s'", resp.Status)
	}

	bcontent, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(bcontent) == 0 {
		return nil, fmt.Errorf("Downloaded version file is empty")
	}

	content := strings.SplitN(strings.TrimSpace(string(bcontent)), "\n", 2)

	content[0] = strings.TrimSpace(content[0])

	if len(content[0]) == 0 {
		return nil, fmt.Errorf("Downloaded version file does not appear to contain a version number")
	}

	if content[0][0] < '0' || content[0][0] > '9' {
		return nil, fmt.Errorf("Downloaded version file does not appear to contain a valid version number starting with a digit")
	}

	return content, nil
}
