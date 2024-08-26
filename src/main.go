package main

import (
	"github.com/gin-gonic/gin"
)

type RequestPayload struct {
	Username string `json:"username"`
	Repo     string `json:"repo"`
}

func main() {
	r := gin.Default()
	r.POST("/garbage", handleGarbageRequest)
	r.Run(":8080")
}

func handleGarbageRequest(c *gin.Context) {
	var payload RequestPayload
	if err := c.BindJSON(&payload); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result, err := ProcessRepository(payload.Username, payload.Repo)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}
