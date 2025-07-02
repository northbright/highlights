package highlights

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/northbright/ffcmd"
	"github.com/northbright/timestamp"
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
	// Add rank prefix(No.XX).
	AddRankPrefix bool    `json:"add_rank_prefix"`
	Out           *Output `json:"output"`
}

func Load(buf []byte) (*Highlights, error) {
	h := &Highlights{}

	if err := json.Unmarshal(buf, h); err != nil {
		return nil, err
	}

	return h, nil
}

func LoadJSON(f string) (*Highlights, error) {
	buf, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}

	return Load(buf)
}

func AddImageClip(ffmpeg *ffcmd.FFmpegCmd, concatFC *ffcmd.FilterChain, ic *ImageClip, name string, out *Output) error {
	if ic.File == "" {
		return fmt.Errorf("image clip's file is empty")
	}

	if name == "" {
		return fmt.Errorf("empty image clip name")
	}

	fps := fmt.Sprintf("fps=%d", out.FPS)
	loop := fmt.Sprintf("loop=loop=%d:size=1", ic.Duration*out.FPS)
	scale := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", out.W, out.H)
	pad := fmt.Sprintf("pad=%d:%d:(ow-iw)/2:(oh-ih)/2", out.W, out.H)
	setsar := "setsar=1:1"
	format := "format=pix_fmts=yuv420p"

	// Create video filterchain.
	v := ffcmd.NewFilterChain(fmt.Sprintf("[%s_v]", name))

	// Add image file as ffmpeg input and get the input index.
	// Add video stream of "h.OP.jpg"([0:v:0]) as op video chain's input.
	v.AddInputByID(ffmpeg.AddInput(ic.File), "v", 0)

	// Chain video filters.
	v.Chain(fps).Chain(loop).Chain(scale).Chain(pad).Chain(setsar).Chain(format)

	// Check if need to chain subtitles filter.
	if ic.Subtitle != "" {
		srtFile := strings.Replace(ic.File, filepath.Ext(ic.File), ".srt", -1)
		createCmd, err := ffcmd.NewCreateOneSubSRTCmdForImageClip(srtFile, ic.Subtitle, float32(ic.Duration))
		if err != nil {
			log.Printf("ffcmd.NewCreateOneSubSRTCmdForImageClip() error: %v", err)
			return err
		}
		// Add command to create SRT file as ffmpeg's pre-commands(set-up commmands).
		ffmpeg.AddPreCmd(createCmd)

		removeCmd, err := ffcmd.NewRemoveOneSubSRTCmd(srtFile)
		if err != nil {
			log.Printf("ffcmd.NewRemoveOneSubSRTCmd() error: %v", err)
			return err
		}
		// Add command to remove created file as ffmpeg's post-commands(clean-up commands).
		ffmpeg.AddPostCmd(removeCmd)

		// Create and chain subtitles filter.
		subtitles := fmt.Sprintf("subtitles='%s':force_style='Fontsize=%d'", srtFile, ic.FontSize)
		v.Chain(subtitles)
	}

	// Chain fade filter.
	fade := fmt.Sprintf("fade=t=out:st=%d:d=%d", ic.Duration-ic.FadeOutDuration, ic.FadeOutDuration)
	v.Chain(fade)

	// Create audio filterchain.
	a := ffcmd.NewFilterChain(fmt.Sprintf("[%s_a]", name))

	// Create audio fiters.
	aevalsrc := fmt.Sprintf("aevalsrc=0:d=%d", ic.Duration)

	// Chain audio filters.
	a.Chain(aevalsrc)

	// Add video / audio filterchain to filtergraph.
	ffmpeg.Chain(v)
	ffmpeg.Chain(a)

	// Add image clip's video and audio filterchain's output as concat filterchain's input.
	concatFC.AddInputByOutput(v, 0)
	concatFC.AddInputByOutput(a, 0)

	return nil
}

func (h *Highlights) FFmpegCmd() (*ffcmd.FFmpegCmd, error) {
	// Create ffmpeg command with output file.
	ffmpeg := ffcmd.NewFFmpegCmd(h.Out.File, true, ffcmd.FFmpegOutputFPS(h.Out.FPS))

	// Create concat filter chain.
	concatFC := ffcmd.NewFilterChain("[outv]", "[outa]")

	// Segments count to concat.
	n := 0

	// Add OP.
	if err := AddImageClip(ffmpeg, concatFC, h.OP, "op", h.Out); err != nil {
		return nil, fmt.Errorf("Add OP error: %v", err)
	}
	n += 1

	clipNum := len(h.Clips)

	// Add video clips.
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
				start, err := timestamp.New(c.Start)
				if err != nil {
					log.Printf("get start timestamp error: %v", err)
					return nil, err
				}
				trim += fmt.Sprintf("start=%s:", start.SecondStr())
				atrim += fmt.Sprintf("start=%s:", start.SecondStr())
			}

			if c.End != "" {
				end, err := timestamp.New(c.End)
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
			text := c.Subtitle
			if h.AddRankPrefix {
				text = fmt.Sprintf("No.%d %s", clipNum-i, c.Subtitle)
			}
			createCmd, err := ffcmd.NewCreateOneSubSRTCmd(srtFile, c.File, text, c.Start, c.End)
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

	// Add ED.
	if err := AddImageClip(ffmpeg, concatFC, h.ED, "ed", h.Out); err != nil {
		return nil, fmt.Errorf("Add ED error: %v", err)
	}
	n += 1

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

func (h *Highlights) Make(ctx context.Context, dir string, stdout, stderr io.Writer) error {
	// Generate FFmpeg command.
	ffmpeg, err := h.FFmpegCmd()
	if err != nil {
		return err
	}

	// Get exec.Cmd
	cmd, err := ffcmd.CommandContext(ctx, ffmpeg)
	if err != nil {
		return err
	}

	// To stop the process and its subprocesses,
	// request that the process group id be set (Setpgid: true) to the PID of the newly spawned process (Pgid: 0).
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	// cmd.Cancel will run when ctx is done.
	cmd.Cancel = func() error {
		// Kill all processes in the group via `kill -9 -$PGID`.
		// Note the "-" to signal the group.
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	if err = cmd.Run(); err != nil {
		return err
	}

	return nil
}
