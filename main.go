package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
	_ "github.com/shirou/gopsutil/v3/process"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

func main() {
	port := ":8080"
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Use(func(context *gin.Context) {
		context.Header("Access-Control-Allow-Origin", "*")
		context.Header("Access-Control-Allow-Headers", "*")
		context.Header("Access-Control-Allow-Methods", "*")
		if context.Request.Method == "OPTIONS" {
			context.Status(200)
			context.Abort()
		}
	})

	installSpeedCLI()
	router.POST("/exportAuthToken", exportNodeType)
	router.POST("/getBalance", getBalance)
	router.POST("/pfb", sendPfb)
	router.GET("/getNodeInfo", getNodeInfo)
	router.GET("/getSamplerStats", getSamplerStats)
	router.GET("/location", location)
	router.GET("/speedInfo", runSpeedCLI)
	router.GET("/getRamCpuMemUsage", getRamCpuMemUsage)
	router.GET("/getCelestiaCpuUsage", getCelestiaCpuUsage)
	fmt.Println("server is running at", port)
	err := router.Run(port)
	if err != nil {
		fmt.Println(err)
		panic(err)
		return
	}
}

type Node struct {
	NodeType string
}

type Response struct {
	JSONRPC string `json:"jsonrpc"`
	Result  Result `json:"result"`
	ID      int    `json:"id"`
}

type Result struct {
	HeadOfSampledChain int  `json:"head_of_sampled_chain"`
	HeadOfCatchup      int  `json:"head_of_catchup"`
	NetworkHeadHeight  int  `json:"network_head_height"`
	Concurrency        int  `json:"concurrency"`
	CatchUpDone        bool `json:"catch_up_done"`
	IsRunning          bool `json:"is_running"`
}

func exportNodeType(c *gin.Context) {
	body := Node{}
	data, err := c.GetRawData()
	if err != nil {
		c.AbortWithStatusJSON(406, "Input format is wrong")
		return
	}
	if err := json.Unmarshal(data, &body); err != nil {
		c.AbortWithStatusJSON(400, "Can't match with the struct")
		return
	}
	fmt.Println(body.NodeType)

	cmd := exec.Command("celestia", body.NodeType, "auth", "admin", "--p2p.network", "blockspacerace")

	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Parse the output to retrieve the token
	token := strings.TrimSpace(string(output))

	// Set the environment variable
	err = os.Setenv("CELESTIA_NODE_AUTH_TOKEN", token)
	if err != nil {
		fmt.Println("Error setting environment variable:", err)
		return
	}
	fmt.Println(token)
	c.JSON(200, gin.H{
		"Token": token,
	})
}

func getSamplerStats(c *gin.Context) {
	cmd := exec.Command("celestia", "rpc", "das", "SamplingStats")
	cmd.Env = append(os.Environ(), "HOME=/root")
	output, err := cmd.Output()
	if err != nil {
		c.AbortWithStatusJSON(400, "Something went wrong with linux command")
		fmt.Println("Error:", err)
		return
	}

	fmt.Println(string(output))

	var resp Response
	err = json.Unmarshal([]byte(output), &resp)
	if err != nil {
		c.AbortWithStatusJSON(400, "Something went wrong with unmarshalling")
		fmt.Println("Error:", err)
		return
	}

	c.JSON(200, gin.H{
		"jsonrpc": resp.JSONRPC,
		"id":      resp.ID,
		"result": gin.H{
			"head_of_sampled_chain": resp.Result.HeadOfSampledChain,
			"head_of_catchup":       resp.Result.HeadOfCatchup,
			"network_head_height":   resp.Result.NetworkHeadHeight,
			"concurrency":           resp.Result.Concurrency,
			"catch_up_done":         resp.Result.CatchUpDone,
			"is_running":            resp.Result.IsRunning,
		},
	})
}

type MyResponse struct {
	JSONRPC string   `json:"jsonrpc"`
	Result  MyResult `json:"result"`
	ID      int      `json:"id"`
}

type MyResult struct {
	Type       int    `json:"type"`
	APIVersion string `json:"api_version"`
}

func getNodeInfo(c *gin.Context) {
	cmd := exec.Command("celestia", "rpc", "das", "SamplingStats")

	output, err := cmd.Output()
	if err != nil {
		c.AbortWithStatusJSON(400, "Something went wrong with linux command")
		fmt.Println("Error:", err)
		return
	}

	fmt.Println(string(output))

	var resp MyResponse
	err = json.Unmarshal([]byte(output), &resp)
	if err != nil {
		c.AbortWithStatusJSON(400, "Something went wrong with unmarshalling")
		fmt.Println("Error:", err)
		return
	}

	c.JSON(200, gin.H{
		"jsonrpc": resp.JSONRPC,
		"id":      resp.ID,
		"result":  gin.H{"type": resp.Result.Type, "api_version": resp.Result.APIVersion},
	})

}

func installSpeedCLI() {
	packageName := "speedtest-cli"

	// Check if the package is already installed
	checkCmd := exec.Command("dpkg", "-s", packageName)
	checkOutput, _ := checkCmd.CombinedOutput()

	if strings.Contains(string(checkOutput), "Status: install ok installed") {
		fmt.Printf("Package '%s' is already installed.\n", packageName)
		return
	}

	// Install the package
	installCmd := exec.Command("apt", "install", packageName)

	output, err := installCmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
		fmt.Println("Command output:", string(output))
		return
	}

	fmt.Println(string(output))
}

type LocationInfo struct {
	Status       string  `json:"status"`
	Country      string  `json:"country"`
	CountryCode  string  `json:"countryCode"`
	Region       string  `json:"region"`
	RegionName   string  `json:"regionName"`
	City         string  `json:"city"`
	Zip          string  `json:"zip"`
	Latitude     float64 `json:"lat"`
	Longitude    float64 `json:"lon"`
	Timezone     string  `json:"timezone"`
	ISP          string  `json:"isp"`
	Organization string  `json:"org"`
	AS           string  `json:"as"`
	Query        string  `json:"query"`
}

func location(c *gin.Context) {
	cmd := exec.Command("curl", "-s", "http://ip-api.com/json")

	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	var locationInfo LocationInfo
	err = json.Unmarshal([]byte(string(output)), &locationInfo)
	if err != nil {
		fmt.Println("Error:", err)
		c.AbortWithStatusJSON(400, "Something went wrong with unmarshalling")

		return
	}
	c.JSON(200, gin.H{
		"Status":       locationInfo.Status,
		"Country":      locationInfo.Country,
		"Country Code": locationInfo.CountryCode,
		"Region":       locationInfo.Region,
		"Region Name":  locationInfo.RegionName,
		"City":         locationInfo.City,
		"ZIP":          locationInfo.Zip,
		"Latitude":     locationInfo.Latitude,
		"Longitude":    locationInfo.Longitude,
		"Timezone":     locationInfo.Timezone,
		"ISP":          locationInfo.ISP,
		"Organization": locationInfo.Organization,
		"AS":           locationInfo.AS,
		"Query":        locationInfo.Query,
	})
}

func extractLocation(output string) (city, region, country string) {
	outputLines := strings.Split(output, "\n")

	if len(outputLines) >= 3 {
		city = strings.Trim(outputLines[0], `"`)
		region = strings.Trim(outputLines[1], `"`)
		country = strings.Trim(outputLines[2], `"`)
	}

	return city, region, country
}
func runSpeedCLI(c *gin.Context) {
	cmd := exec.Command("speedtest-cli")

	output, err := cmd.CombinedOutput()
	if err != nil {
		c.AbortWithStatusJSON(400, "Something went wrong with linux command")
		return
	}

	// Extract upload and download speed values
	reDownload := regexp.MustCompile(`Download:\s+(\d+\.\d+)\s+([KM]bit/s)`)
	reUpload := regexp.MustCompile(`Upload:\s+(\d+\.\d+)\s+([KM]bit/s)`)
	downloadSpeed := reDownload.FindStringSubmatch(string(output))
	uploadSpeed := reUpload.FindStringSubmatch(string(output))
	var uploadSpeedVal string
	var downloadSpeedVal string

	if len(downloadSpeed) == 3 {
		downloadValue := downloadSpeed[1]
		downloadUnit := downloadSpeed[2]
		downloadSpeedVal = downloadValue + downloadUnit
	} else {

		fmt.Println("Failed to extract download speed.")
	}

	if len(uploadSpeed) == 3 {
		uploadValue := uploadSpeed[1]
		uploadUnit := uploadSpeed[2]
		uploadSpeedVal = uploadValue + uploadUnit
		fmt.Println("Upload Speed:", uploadValue, uploadUnit)
	} else {
		fmt.Println("Failed to extract upload speed.")
	}

	c.JSON(200, gin.H{
		"download": downloadSpeedVal,
		"upload":   uploadSpeedVal,
	})
}

func getRamCpuMemUsage(c *gin.Context) {
	v, err := mem.VirtualMemory()
	if err != nil {
		fmt.Println("err ram usage", err)
	}

	totalRAM := math.Round(float64(v.Total)/(1024*1024*1024)*100) / 100
	usedRAM := math.Round(float64(v.Used)/(1024*1024*1024)*100) / 100
	totalCPU := runtime.NumCPU()

	c.JSON(200, gin.H{
		"totalRam": totalRAM,
		"usedRam":  usedRAM,
		"totalCPU": totalCPU,
	})

}

func getCelestiaCpuUsage(c *gin.Context) {
	processes, err := process.Processes()
	if err != nil {
		fmt.Println("err Cpu Celestia", err)
		return
	}

	for _, proc := range processes {
		name, err := proc.Name()
		if err != nil {
			continue
		}

		if name == "celestia" {
			cpuPercent, err := proc.CPUPercent()
			if err != nil {
				fmt.Println("err Cpu Celestia", err)
			}
			c.JSON(200, gin.H{
				"cpuPercent": math.Round(cpuPercent) / 100,
			})

			break
		}
	}
}

type getBalanceSt struct {
	IpAddress string
}

type BalanceResponse struct {
	Denom  string
	Amount string
}

func getBalance(c *gin.Context) {

	bodyIp := getBalanceSt{}

	rawInput, err := c.GetRawData()
	if err != nil {
		c.AbortWithStatusJSON(406, "Invalid input format")
		return
	}
	if err := json.Unmarshal(rawInput, &bodyIp); err != nil {
		c.AbortWithStatusJSON(400, "Struct mismatch")
		return
	}

	url := fmt.Sprintf("http://%s:26659/balance", bodyIp.IpAddress)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error making the request:", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading the response:", err)
		return
	}

	var balanceResponse BalanceResponse
	err = json.Unmarshal(body, &balanceResponse)
	if err != nil {
		fmt.Println("Error unmarshalling JSON:", err)
		return
	}

	c.JSON(200, gin.H{
		"denom":  balanceResponse.Denom,
		"amount": balanceResponse.Amount,
	})

}

type PFB struct {
	NamespaceId string `json:"namespace_id"`
	Data        string `json:"data"`
	GasLimit    int    `json:"gas_limit"`
	Fee         int    `json:"fee"`
	IpAddress   string `json:"ip_address"`
}

type ResponseData struct {
	Height    int64  `json:"height"`
	TxHash    string `json:"txhash"`
	Data      string `json:"data"`
	RawLog    string `json:"raw_log"`
	GasWanted int64  `json:"gas_wanted"`
	GasUsed   int64  `json:"gas_used"`
	Events    []struct {
		Type       string `json:"type"`
		Attributes []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
			Index bool   `json:"index"`
		} `json:"attributes"`
	} `json:"events"`
}

func sendPfb(c *gin.Context) {
	body := PFB{}
	rawInput, err := c.GetRawData()
	if err != nil {
		c.AbortWithStatusJSON(406, "Invalid input format")
		return
	}
	if err := json.Unmarshal(rawInput, &body); err != nil {
		c.AbortWithStatusJSON(400, "Struct mismatch")
		return
	}

	pfb := PFB{
		NamespaceId: body.NamespaceId,
		Data:        body.Data,
		GasLimit:    body.GasLimit,
		Fee:         body.Fee,
	}

	jsonData, err := json.Marshal(pfb)
	if err != nil {
		fmt.Println("Error encoding JSON:", err)
		return
	}

	url := fmt.Sprintf("http://%s:26659/submit_pfb", body.IpAddress)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Error making the request:", err)
		c.AbortWithStatusJSON(400, "Make sure you have enough Tia use our tool to check it out")
		return
	}
	defer resp.Body.Close()

	bodyHead, err := ioutil.ReadAll(resp.Body)
	responseData := ResponseData{}
	err = json.Unmarshal(bodyHead, &responseData)
	if err != nil {
		fmt.Println("Error parsing the JSON response:", err)
		c.AbortWithStatusJSON(400, "Make sure you have enough Tia use our tool to check it out")
		return
	}

	c.JSON(200, responseData)

}
