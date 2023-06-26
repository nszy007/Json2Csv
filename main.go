package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ecoshub/jin"
	"github.com/gogf/gf/container/gmap"
)

func readJSONFile(bJsonFile []byte) (*gmap.ListMap, error) {
	jsonListMap := gmap.NewListMap(true)
	if err := json.Unmarshal(bJsonFile, &jsonListMap); err != nil {
		return nil, err
	}
	return jsonListMap, nil
}

func getMaxNode(listMap *gmap.ListMap) string {
	maxSize := 0
	maxNode := ""

	listMap.Iterator(func(key interface{}, value interface{}) bool {
		if subList, ok := value.([]interface{}); ok {
			if len(subList) > maxSize {
				maxSize = len(subList)
				maxNode = key.(string)
			}
		}
		return true
	})

	return maxNode
}

// 兼容非数组对象数据提取{"items":{"a1":{"title":"one","name":"test"},"b2":{"title":"two","name":"test2"}}}
func writeObjToCSVFile(obj []string, csvHeader []string, outputFileName string) error {
	file, err := os.Create(outputFileName)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	writer.Write(csvHeader)
	for _, objItem := range obj {
		var record map[string]interface{}
		json.Unmarshal([]byte(objItem), &record)
		var row []string

		for _, k := range csvHeader {
			switch record[k].(type) {
			case []interface{}:
				data := record[k].([]interface{})
				str := ""
				for i := 0; i < len(data); i++ {
					str1 := fmt.Sprintf("%v", data[i])
					str += str1 + ","
				}
				row = append(row, str[:len(str)-1])
			case string:
				row = append(row, record[k].(string))
			case nil:
				row = append(row, "")
			default:
				dataType, _ := json.Marshal(record[k])
				row = append(row, string(dataType))
			}
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

func writeArrayToCSVFile(listMap *gmap.ListMap, node string, csvHeader []string, outputFileName string) error {
	data := listMap.Get(node)
	arr, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf(" > 数据区域不是一个数组！")
	}
	file, err := os.Create(outputFileName)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	writer.Write(csvHeader)
	for _, item := range arr {
		if record, ok := item.(map[string]interface{}); ok {
			var row []string

			// 注意Map和json默认无序，会导致csv输出时列不对齐
			// 上面保留key顺序，然后从Map中按key顺序取值以保证value有序
			for _, k := range csvHeader {
				//fmt.Println(reflect.TypeOf(record[k]))
				switch record[k].(type) {
				case []interface{}:
					//fmt.Println(record[k])
					data := record[k].([]interface{})
					str := ""
					for i := 0; i < len(data); i++ {
						str1 := fmt.Sprintf("%v", data[i])
						str += str1 + ","
					}
					row = append(row, str[:len(str)-1])
				case string:
					row = append(row, record[k].(string))
				case nil:
					row = append(row, "")
				default:
					dataType, _ := json.Marshal(record[k])
					row = append(row, string(dataType))
				}
			}

			if err := writer.Write(row); err != nil {
				return err
			}
		}
	}

	return nil
}

func isFileExist(filename string) bool {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false
	}
	return true
}

func process(jPath string, szkey string) {
	if isFileExist(jPath) == false {
		fmt.Printf(" > 指定文件不存在：%s\n", jPath)
		return
	}
	csvFilePath := strings.Split(jPath, filepath.Ext(jPath))[0] + ".csv"
	bJsonFile, err := ioutil.ReadFile(jPath)
	if err != nil {
		return
	}

	// 如果设置-k参数，则按szkey指定路径提取json
	if len(szkey) > 0 {
		path := []string{}
		for _, v := range strings.Split(szkey, ".") {
			if len(v) > 0 {
				path = append(path, strings.TrimSpace(v))
			}
		}

		bJsonFile, err = jin.Get(bJsonFile, path...)
		if err != nil {
			fmt.Printf(" > %s 读取错误： %v\n", jPath, err)
			fmt.Printf(" > 请对照 %s 文件检查-k参数路径！\n", jPath)
			return
		}
	}

	// 兼容处理[{"ID":0,"Name":"Lucy"},{"ID":1,"Name":"Lily"}]类型
	if bJsonFile[0] == '[' {
		bJsonFile = bytes.Join([][]byte{[]byte("{\"兼容\":"), bJsonFile, []byte("}")}, []byte(""))
	}

	listMap, err := readJSONFile(bJsonFile)
	if err != nil {
		fmt.Printf(" > %s 读取错误： %v\n", jPath, err)
		return
	}

	var csvHeader, objValues []string
	maxNode := getMaxNode(listMap)
	// csv表头仅首行写入一次；用于后续保留key顺序
	if maxNode == "" {
		// 兼容处理非数组对象数据提取{"items":{"a1":{"title":"one","name":"test"},"b2":{"title":"two","name":"test2"}}}
		objValues, err = jin.GetValues(bJsonFile)
		if err != nil {
			fmt.Println(err)
			return
		}
		csvHeader, err = jin.GetKeys([]byte(objValues[0]))
	} else {
		fmt.Printf(" > 数据节点： %s\n", maxNode)
		// 因为map和json都无序，想保留json键顺序采用第三方库jin
		csvHeader, err = jin.GetKeys(bJsonFile, maxNode, "0")
	}
	if err != nil {
		fmt.Printf(" > %s 读取错误： %v\n", jPath, err)
		flag.Usage()
		return
	}
	fmt.Printf(" > %s 字段列表： %v\n", jPath, csvHeader)

	if maxNode == "" {
		err = writeObjToCSVFile(objValues, csvHeader, csvFilePath)
	} else {
		err = writeArrayToCSVFile(listMap, maxNode, csvHeader, csvFilePath)
	}
	if err != nil {
		fmt.Printf(" > CSV文件写入错误： %v\n\n", err)
	} else {
		fmt.Printf(" > CSV文件成功写入： %s\n\n", csvFilePath)
	}
}

var (
	bhelp    bool
	bVersion bool
	szkey    string
)

func init() {
	flag.BoolVar(&bhelp, "h", false, "显示帮助")
	flag.BoolVar(&bVersion, "v", false, "显示版本信息")
	flag.StringVar(&szkey, "k", "", "设置Json中数据所处路径，如'-k root.topics.data'")
}

func main() {
	flag.Parse()
	if bhelp {
		flag.Usage()
		return
	}
	if bVersion {
		fmt.Println(" > 版本：v0.2\n > 主页：https://github.com/playGitboy/Json2Csv")
		return
	}

	if flag.NArg() > 0 {
		for _, jsonFilePath := range flag.Args() {
			process(jsonFilePath, szkey)
		}
	} else {
		fmt.Println(" > Json2Csv：请指定JSON格式文件路径（支持批量）...")
		fmt.Println(" > Json2Csv [-k root.data.items] data.json data2.txt ...")
		flag.Usage()
	}
	//fmt.Scanln()
}
