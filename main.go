package main

import (
    "context"
    "fmt"
    "net/http"
	"os"
	"os/signal"
	"time"
	// "encoding/json"
    "path/filepath"
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/clientcmd"
    // "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/client-go/util/homedir"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)
//定义集群配置变量类型
var cfg *rest.Config
// 获取自定义资源实例
var	gvr = schema.GroupVersionResource{
		Group:    "core.k8sgpt.ai",
		Version:  "v1alpha1",
		Resource: "results",
	}

var gauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "k8sgpt_diagnostic_results",
				Help: "Diagnostic results from Kubernetes",
			},
			[]string{"resultname", "kind", "podname", "errorinfo", "time"},
	)
var server *http.Server

func registerMetricsEndpoint(cfg *rest.Config, namespace string){
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request){
		gauge.Reset()
		
		// 创建动态客户端以操作自定义资源
		dynClient, err := dynamic.NewForConfig(cfg)
		if err != nil {
			panic(err)
		}

		// 使用动态客户端获取资源列表
		resourceClient := dynClient.Resource(gvr).Namespace(namespace)
		unstructuredList, err := resourceClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				fmt.Println("Results resource not found")
			} else {
				panic(err)
			}
		}
		// 遍历结果资源列表并打印名称
		for _, item := range unstructuredList.Items {
			resultName := item.GetName()
			resultResource := dynClient.Resource(gvr).Namespace(namespace)
			result, err := resultResource.Get(context.TODO(), resultName, metav1.GetOptions{})
			if err != nil {
				panic(err)
			}

			// 解析资源详情，查找 spec.error.text 字段
			resultData, ok := result.Object["spec"].(map[string]interface{})
			if !ok {
				fmt.Println("'spec' field not found or is not a map[string]interface{}")
				return
			}

			errorsList, ok := resultData["error"].([]interface{})
			if !ok {
				fmt.Println("'error' field not found in 'spec' or is not an array")
				return
			}

			kind_value, ok := resultData["kind"].(string)
			if !ok {
				fmt.Println("'kind' field not found in 'spec' or is not an array")
				return
			}

			podname,ok := resultData["name"].(string)
			if !ok {
				fmt.Println("'name' field not found in 'spec' or is not an array")
				return
			}

			for _, errorItem := range errorsList {
				errorMap, ok := errorItem.(map[string]interface{})
				if !ok {
					continue
				}

				errorinfo, ok := errorMap["text"].(string)
				if ok {
					gauge.WithLabelValues(resultName, kind_value, podname, errorinfo, time.Now().Format(time.RFC3339)).Set(0)
					fmt.Printf("k8sgpt_diagnostic_results{resultname=\"%s\",kind=\"%s\",podname=\"%s\",errorinfo=\"%s\", time=\"%s\"} 0\n", resultName, kind_value, podname, errorinfo, time.Now().Format(time.RFC3339))
				}
			}
		}
		//
		promhttp.Handler().ServeHTTP(w, r)
	})
}
func startHTTPServer(){
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, this is a sample endpoint.")
	})
	http.ListenAndServe(":8080", nil)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			fmt.Println(err)
		}
	}()
}	

func main(){
	//根据环境变量获取需要查询的资源所属的命名空间
	namespace := os.Getenv("RESULTSNAMESPACE")
	if namespace == " " {
		fmt.Println("Namespace environment variable not found")
	} else {
		fmt.Println("Namespace value:", namespace)
	}
    //检查是否存在主目录
	home := homedir.HomeDir()
	if home == "" {
		fmt.Println("Error getting home directory")
	}
	//检查是否主目录是否存在.kube/config集群配置文件
	kubeconfig := filepath.Join(home, ".kube", "config")
	//获取集群中.kube/config配置文件信息
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		fmt.Println(".kube/config does not exist, using in-cluster config")
		cfg, _ = rest.InClusterConfig()
	} else {
		fmt.Println("Using kubeconfig from", kubeconfig)
		cfg, _ = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	fmt.Println("集群配置：")
	fmt.Println(cfg)
	//注册gauge指标
	prometheus.MustRegister(gauge)
	registerMetricsEndpoint(cfg, namespace)
	startHTTPServer()
	// Wait for interrupt signal to gracefully shutdown the server
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	// Shutdown the server
	fmt.Println("Shutting down the server...")
	if err := server.Shutdown(context.Background()); err != nil {
		fmt.Println(err)
	}
	fmt.Println("Server gracefully stopped")
}
