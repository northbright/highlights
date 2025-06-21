package highlights

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"github.com/northbright/ffcmd"
)

type ImageClip struct {
	File            string `json:"file"`
	Duration        int    `json:"duration"`
	FadeOutDuration int    `json:"fade_out_duration"`
	Subtitle        string `json:"subtitle"`
	FontSize        int    `json:"font_size"`
}

type Clip struct {
	File     string `json:"file"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Subtitle string `json:"subtitle"`
	FontSize int    `json:"font_size"`
}

type Output struct {
	File string `json:"file"`
	W    int    `json:"w"`
	H    int    `json:"h"`
	FPS  int    `json:"fps"`
}

type Highlights struct {
	OP    *ImageClip `json:"op"`
	ED    *ImageClip `json:"ed"`
	Clips []*Clip    `json:"clips"`
	BGM   string     `json:"bgm"`
	Out   Output     `json:"output"`
}

func Load(buf []byte) (*Highlights, error) {
	h := &Highlights{}

	if err := json.Unmarshal(buf, h); err != nil {
		return nil, err
	}

	return h, nil
}

func LoadJSON(f string) (*Highlights, error) {
	buf, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, err
	}

	return Load(buf)
}

func (h *Highlights) GenerateFFmpegCmd() (*ffcmd.FFmpeg, error) {
	// Create ffmpeg command with output file.
	ffmpeg := ffcmd.New(h.Out.File, true)

	// Create op video filterchain.
	op_v := ffcmd.NewFilterChain("[op_v]")

	// Add "h.OP.jpg" as ffmpeg input and get the input index.
	// Add video stream of "h.OP.jpg"([0:v:0]) as op video chain's input.
	op_v.AddInputByID(ffmpeg.AddInput(h.OP.File), "v", 0)

	// Create op video filters.
	fps := fmt.Sprintf("fps=%d", h.Out.FPS)
	loop := fmt.Sprintf("loop=loop=%d:size=1", h.OP.Duration*h.Out.FPS)
	scale := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", h.Out.W, h.Out.H)
	pad := fmt.Sprintf("pad=%d:%d:(ow-iw)/2:(oh-ih)/2", h.Out.W, h.Out.H)
	setsar := "setsar=1:1"
	format := "format=pix_fmts=yuv420p"

	// Chain op video filters.
	op_v.Chain(fps).Chain(loop).Chain(scale).Chain(pad).Chain(setsar).Chain(format)

	// Check if need to chain subtitles filter.
	if h.OP.Subtitle != "" {
		srtFile := strings.Replace(h.OP.File, filepath.Ext(h.OP.File), ".srt", -1)
		createCmd, err := ffcmd.NewCreateOneSubSRTCmdForImageClip(srtFile, h.OP.Subtitle, float32(h.OP.Duration))
		if err != nil {
			log.Printf("ffcmd.NewCreateOneSubSRTCmdForImageClip() error: %v", err)
			return nil, err
		}
		// Add command to create SRT file as ffmpeg's pre-commands(set-up commmands).
		ffmpeg.AddPreCmd(createCmd)

		removeCmd, err := ffcmd.NewRemoveOneSubSRTCmd(srtFile)
		if err != nil {
			log.Printf("ffcmd.NewRemoveOneSubSRTCmd() error: %v", err)
			return nil, err
		}
		// Add command to remove created file as ffmpeg's post-commands(clean-up commands).
		ffmpeg.AddPostCmd(removeCmd)

		// Create and chain subtitles filter.
		subtitles := fmt.Sprintf("subtitles='%s':force_style='Fontsize=%d'", srtFile, h.OP.FontSize)
		op_v.Chain(subtitles)
	}

	// Chain fade filter.
	fade := fmt.Sprintf("fade=t=out:st=%d:d=%d", h.OP.Duration-h.OP.FadeOutDuration, h.OP.FadeOutDuration)
	op_v.Chain(fade)

	// Create op audio filterchain.
	op_a := ffcmd.NewFilterChain("[op_a]")

	// Create op audio fiters.
	aevalsrc := fmt.Sprintf("aevalsrc=0:d=%d", h.OP.Duration)

	// Chain ed audio filters.
	op_a.Chain(aevalsrc)

	// Add op video / audio filterchain to filtergraph.
	ffmpeg.Chain(op_v)
	ffmpeg.Chain(op_a)

	// Create ed video filterchain.
	ed_v := ffcmd.NewFilterChain("[ed_v]")

	// Add "h.ED.jpg" as ffmpeg input and get the input index.
	// Add video stream of "h.ED.jpg"([1:v:0]) as ed's input.
	ed_v.AddInputByID(ffmpeg.AddInput(h.ED.File), "v", 0)

	// Create ed video filters.
	loop = fmt.Sprintf("loop=loop=%d:size=1", h.ED.Duration*h.Out.FPS)

	// Chain ed video filters.
	ed_v.Chain(fps).Chain(loop).Chain(scale).Chain(pad).Chain(setsar).Chain(format)

	// Check if need to chain subtitles filter.
	if h.ED.Subtitle != "" {
		srtFile := strings.Replace(h.ED.File, filepath.Ext(h.ED.File), ".srt", -1)
		createCmd, err := ffcmd.NewCreateOneSubSRTCmdForImageClip(srtFile, h.ED.Subtitle, float32(h.ED.Duration))
		if err != nil {
			log.Printf("ffcmd.NewCreateOneSubSRTCmdForImageClip() error: %v", err)
			return nil, err
		}
		// Add command to create SRT file as ffmpeg's pre-commands(set-up commmands).
		ffmpeg.AddPreCmd(createCmd)

		removeCmd, err := ffcmd.NewRemoveOneSubSRTCmd(srtFile)
		if err != nil {
			log.Printf("ffcmd.NewRemoveOneSubSRTCmd() error: %v", err)
			return nil, err
		}
		// Add command to remove created file as ffmpeg's post-commands(clean-up commands).
		ffmpeg.AddPostCmd(removeCmd)

		// Create and chain subtitles filter.
		subtitles := fmt.Sprintf("subtitles='%s':force_style='Fontsize=%d'", srtFile, h.ED.FontSize)
		ed_v.Chain(subtitles)
	}

	// Chain fade filter.
	fade = fmt.Sprintf("fade=t=out:st=%d:d=%d", h.ED.Duration-h.ED.FadeOutDuration, h.ED.FadeOutDuration)
	ed_v.Chain(fade)

	// Create audio filterchain.
	ed_a := ffcmd.NewFilterChain("[ed_a]")

	// Create ed audio fiters.
	aevalsrc = fmt.Sprintf("aevalsrc=0:d=%d", h.ED.Duration)

	// Chain ed audio filters.
	ed_a.Chain(aevalsrc)

	// Add ed video / audio filterchain to filtergraph.
	ffmpeg.Chain(ed_v)
	ffmpeg.Chain(ed_a)

	// Create concat filter chain.
	concatFC := ffcmd.NewFilterChain("[outv]", "[outa]")

	// Add op video and audio filterchain's output as concat filterchain's input.
	concatFC.AddInputByOutput(op_v, 0)
	concatFC.AddInputByOutput(op_a, 0)

	// Segments count to concat.
	// Initialized to 2: op + h.ED.
	n := 2

	// Loop all video clips.
	for i, c := range h.Clips {
		// Create clip video filter chain.
		clip_v := ffcmd.NewFilterChain(fmt.Sprintf("[clip_%02d_v]", i))

		// Create clip audio filter chain.
		clip_a := ffcmd.NewFilterChain(fmt.Sprintf("[clip_%02d_a]", i))

		// Add video file as ffmpeg input and get the input index.
		// Add video / audio stream of the file([X:v:0] / [X:a:0], X is the ffmpeg input id) as clip's input.
		id := ffmpeg.AddInput(c.File)
		clip_v.AddInputByID(id, "v", 0)
		clip_a.AddInputByID(id, "a", 0)

		// Create and chain scale, pad, setsar filters.
		scale := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", h.Out.W, h.Out.H)
		pad := fmt.Sprintf("pad=%d:%d:(ow-iw)/2:(oh-ih)/2", h.Out.W, h.Out.H)
		setsar := "setsar=1:1"

		clip_v.Chain(scale).Chain(pad).Chain(setsar)

		// Check if need to chain trim, setpts / atrim, asetpts filter.
		if c.Start != c.End {
			// Create clip video / audio filters.
			trim := "trim="
			atrim := "atrim="

			if c.Start != "" {
				start, err := ffcmd.NewTimestamp(c.Start)
				if err != nil {
					log.Printf("get start timestamp error: %v", err)
					return nil, err
				}
				trim += fmt.Sprintf("start=%s:", start.SecondStr())
				atrim += fmt.Sprintf("start=%s:", start.SecondStr())
			}

			if c.End != "" {
				end, err := ffcmd.NewTimestamp(c.End)
				if err != nil {
					log.Printf("get end timestamp error: %v", err)
					return nil, err
				}
				trim += fmt.Sprintf("end=%s", end.SecondStr())
				atrim += fmt.Sprintf("end=%s", end.SecondStr())
			} else {
				trim = strings.TrimSuffix(trim, ":")
				atrim = strings.TrimSuffix(atrim, ":")
			}

			setpts := "setpts=PTS-STARTPTS"

			// Chain trim and setpts filter.
			clip_v.Chain(trim).Chain(setpts)

			asetpts := "asetpts=PTS-STARTPTS"

			// Chain atrim and asetpts filter.
			clip_a.Chain(atrim).Chain(asetpts)
		}

		// Check if need to chain subtitles filter.
		if c.Subtitle != "" {
			srtFile := strings.Replace(c.File, filepath.Ext(c.File), ".srt", -1)
			createCmd, err := ffcmd.NewCreateOneSubSRTCmd(srtFile, c.File, c.Subtitle, c.Start, c.End)
			if err != nil {
				log.Printf("ffcmd.NewCreateOneSubSRTCmd() error: %v", err)
				return nil, err
			}
			// Add command to create SRT file as ffmpeg's pre-commands(set-up commmands).
			ffmpeg.AddPreCmd(createCmd)

			removeCmd, err := ffcmd.NewRemoveOneSubSRTCmd(srtFile)
			if err != nil {
				log.Printf("ffcmd.NewRemoveOneSubSRTCmd() error: %v", err)
				return nil, err
			}
			// Add command to remove created file as ffmpeg's post-commands(clean-up commands).
			ffmpeg.AddPostCmd(removeCmd)

			// Create and chain subtitles filter.
			subtitles := fmt.Sprintf("subtitles='%s':force_style='Fontsize=%d'", srtFile, c.FontSize)
			clip_v.Chain(subtitles)
		}

		// Add clip video / audio filterchain to filtergraph.
		ffmpeg.Chain(clip_v)
		ffmpeg.Chain(clip_a)

		// Add clip video / audio filter chain's output as concat filterchain's input.
		concatFC.AddInputByOutput(clip_v, 0)
		concatFC.AddInputByOutput(clip_a, 0)

		// Increase segment count.
		n += 1
	}

	// Add ed video and audio filterchain's output as concat filterchain's input.
	concatFC.AddInputByOutput(ed_v, 0)
	concatFC.AddInputByOutput(ed_a, 0)

	// Create concat filters.
	concat := fmt.Sprintf("concat=n=%d:v=1:a=1", n)

	// Chain concat filters.
	concatFC.Chain(concat)

	// Add concat filterchain to filtergraph.
	ffmpeg.Chain(concatFC)

	// Add BGM as command input.
	id := ffmpeg.AddInput(h.BGM)

	// Create filterchain to merge BGM and original audio streams.
	bgmFC := ffcmd.NewFilterChain("[outa_merged_bgm]")
	bgmFC.AddInputByID(id, "a", 0)
	bgmFC.AddInputByOutput(concatFC, 1)

	// Create amerge filter.
	amerge := "amerge=inputs=2"

	// Create pan filter.
	pan := "pan=stereo|c0<c0+c2|c1<c1+c3"

	// Chain filters.
	bgmFC.Chain(amerge).Chain(pan)

	// Add BGM filterchain.
	ffmpeg.Chain(bgmFC)

	// Select output streams.
	// If none stream is selected, it'll auto select last filterchain's labeled outputs.
	ffmpeg.MapByOutput(concatFC, 0)
	ffmpeg.MapByOutput(bgmFC, 0)

	return ffmpeg, nil
}

func (h *Highlights) Make(dir string, stdout, stderr io.Writer) error {
	// Generate FFmpeg command.
	ffmpeg, err := h.GenerateFFmpegCmd()
	if err != nil {
		return err
	}

	// Get exec.Cmd
	cmd, err := ffmpeg.Command()
	if err != nil {
		return err
	}

	// Run
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	log.Printf("cmd.String(): %s", cmd.String())
	if err = cmd.Run(); err != nil {
		return err
	}

	return nil
}
