package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/Unknwon/goconfig"
	"gopkg.in/redis.v5"
)

//IMAGE value
type IMAGE struct {
	Height int
	Width  int
	Src    string
}

//conf 配置文件
var conf *goconfig.ConfigFile

//Image value
var Image IMAGE

func main() {
	conf, _ = goconfig.LoadConfigFile("conf.ini")
	redisClients := getRedis()
	uploadPath := "./uploads/"       //用户视频
	picPath := "./template/picture/" //图片模板
	musicPath := "./template/music/" //音乐模板
	workerPath := "./worker/"
	exportPath := "./fixed/"
	for {
		task, _ := redisClients.BLPop(0, "tasks").Result()
		uuid := ""
		pic := ""        //图片
		background := "" //背景音乐
		// pic := picPath + "pic.jpg"
		// background := musicPath + "sound.mp3"
		if len(task) > 1 {
			mixedTask := task[1]
			s := strings.Split(mixedTask, ",")
			if len(s) == 3 {
				uuid = s[0]
				pic = picPath + s[1] + ".jpg"
				background = musicPath + s[2] + ".mp3"
			} else {
				continue
			}
		} else {
			continue
		}
		//从redis里面读取，一条一条处理，处理完成之后调用

		//文件
		file := uuid + ".mp4"
		filePath := uploadPath + file
		ext := path.Ext(file)
		filename := strings.TrimSuffix(file, ext)
		//拆分成视频和音频
		fileVideo := workerPath + filename + "_video_nosound" + ext
		fileAudio := workerPath + filename + ".aac"
		//混合图片和视频
		fileMixedVideo := workerPath + filename + "_video_mixed" + ext
		//两个音频合成结果
		fileAudio2 := workerPath + filename + "_done" + ".aac" //合成后的音频
		//成果
		fileDone := exportPath + file

		fmt.Println("检查视频是否合规")
		if !videoCheck(filePath) {
			fmt.Println(filePath, "竖屏视频暂不支持")
			continue
		}

		fmt.Println("开始合成", pic, filePath)
		imageInit(pic)

		makeVideoAndAudio(filePath, fileVideo, fileAudio)

		mixPicAndVideo(Image, fileVideo, fileMixedVideo)

		mixVioceAndBackground(fileAudio, background, fileAudio2)

		mixVideoAndAudio(fileMixedVideo, fileAudio2, fileDone)

		clearFile(fileVideo, fileAudio, fileMixedVideo, fileAudio2)

		fmt.Println("输出成果", fileDone)
	}
}

func getRedis() *redis.Client {
	sec, err := conf.GetSection("redis")
	if err != nil {
		fmt.Println("connect redis error :", err)
	}
	maxActive, _ := strconv.Atoi(sec["maxActive"])
	return redis.NewClient(&redis.Options{
		Addr:     sec["address"],
		Password: sec["password"],
		DB:       0,
		PoolSize: maxActive,
	})
}

func noticeWechat() {
	addr := "https://sc.ftqq.com/xxxx.send?text=生成了"
	client := &http.Client{}
	req, err := http.NewRequest("GET", addr, nil) //建立一个请求
	if err != nil {
		fmt.Println("Fatal error ", err.Error())
		return
	}
	client.Do(req)
}

func clearFile(fileVideo string, fileAudio string, fileMixedVideo string, fileAudio2 string) {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("rm -f %s", fileVideo))
	cmd.CombinedOutput()
	cmd = exec.Command("sh", "-c", fmt.Sprintf("rm -f %s", fileAudio))
	cmd.CombinedOutput()
	cmd = exec.Command("sh", "-c", fmt.Sprintf("rm -f %s", fileMixedVideo))
	cmd.CombinedOutput()
	cmd = exec.Command("sh", "-c", fmt.Sprintf("rm -f %s", fileAudio2))
	cmd.CombinedOutput()
}

func imageInit(src string) {
	Image.Src = src
	f, _ := os.Open(src)
	defer f.Close()
	c, _, _ := image.DecodeConfig(f)
	Image.Height = c.Height
	Image.Width = c.Width
	// fmt.Println("image: ", Image)
}

//检查视频是否横屏拍摄，角度，如果不对还需要转换成正常角度
func videoCheck(file string) bool {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("ffprobe -show_streams %s -hide_banner", file))
	out, _ := cmd.CombinedOutput()
	outData := string(out)
	// heightExp := regexp.MustCompile(`height=(\d+)`)
	// widthExp := regexp.MustCompile(`width=(\d+)`)

	// heights := heightExp.FindStringSubmatch(outData)
	// height := "0"
	// width := "0"

	// if len(heights) > 1 {
	// 	height = heights[1]
	// }
	// widths := widthExp.FindStringSubmatch(outData)
	// if len(widths) > 1 {
	// 	width = widths[1]
	// }
	rotate := "0"
	rotateExp := regexp.MustCompile(`rotate=(\d+)`)
	rotates := rotateExp.FindStringSubmatch(outData)
	if len(rotates) > 1 {
		rotate = rotates[1]
	}
	// fmt.Println(height, width, rotate)
	if r, _ := strconv.Atoi(rotate); r > 0 {
		return false
	}
	return true
}

func makeVideoAndAudio(file string, video string, audio string) {
	// fmt.Println("转换成视频文件", video, "音频文件", audio)
	//cmd := exec.Command("sh", "-c", "ffmpeg -i sp.mp4 -vcodec copy -an sp_video.mp4 -acodec copy -vn sp_audio.aac")
	cmd := exec.Command("sh", "-c", fmt.Sprintf("ffmpeg -i %s -vcodec copy -an %s -acodec copy -vn %s", file, video, audio))
	_, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	// fmt.Println(string(out))
	return
}

func mixPicAndVideo(image IMAGE, video string, videoNoSound string) {
	// fmt.Println("图片和视频合成")
	//cmd := exec.Command("sh", "-c", `ffmpeg -i 2020.jpg -an -i sp_video.mp4 -filter_complex "[1:v]scale=w=1080:h=640:force_original_aspect_ratio=decrease[ckout];[0:v][ckout]overlay=x=0:y=300[out]" -map "[out]" -movflags faststart sp_no_audio.mp4`)
	//计算视频位置
	y := image.Height/2 - 320
	// fmt.Println("y", y)
	//strconv.Itoa(y)
	c := fmt.Sprintf(`ffmpeg -i %s -an -i %s -filter_complex "[1:v]scale=w=1080:h=640:force_original_aspect_ratio=decrease[ckout];[0:v][ckout]overlay=x=0:y=%s[out]" -map "[out]" -movflags faststart %s`, image.Src, video, strconv.Itoa(y), videoNoSound)
	// fmt.Println(c)
	cmd := exec.Command("sh", "-c", c)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return
	}
	// fmt.Println("Result: " + out.String())
	return
}

func mixVideoAndAudio(video string, audio string, export string) {
	// fmt.Println("视频和音频合成")
	cmd := exec.Command("sh", "-c", fmt.Sprintf("ffmpeg -y -i %s -i %s %s", video, audio, export))
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return
	}
	// fmt.Println("Result: " + out.String())
	return
}

func mixVioceAndBackground(audio1 string, audio2 string, export string) {
	// fmt.Println("音频和音频合成")
	cmd := exec.Command("sh", "-c", fmt.Sprintf("ffmpeg -i %s -i %s -filter_complex amix=inputs=2:duration=first:dropout_transition=2 %s", audio1, audio2, export))
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return
	}
	// fmt.Println("Result: " + out.String())
	return
}
