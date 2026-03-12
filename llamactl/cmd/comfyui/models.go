package comfyui

// models.go — "llamactl comfyui models" subcommand.
// Lists available image-generation models and lets the user download them
// interactively using wget/curl into the correct ComfyUI directory.

import (
"fmt"
"os"
"os/exec"
"path/filepath"

"github.com/andermurias/llamactl/internal/config"
"github.com/pterm/pterm"
"github.com/spf13/cobra"
)

// modelEntry describes a downloadable image-generation model.
type modelEntry struct {
Name    string
Short   string // one-line description
Size    string
URL     string
SubDir  string // relative to ComfyUI/models/
File    string // target filename
}

// catalog lists curated, freely-downloadable models that work on Apple Silicon.
var catalog = []modelEntry{
{
Name:   "FLUX.1-schnell (fp8)",
Short:  "Fast 12B model — best quality/speed on M-series",
Size:   "~9 GB",
URL:    "https://huggingface.co/black-forest-labs/FLUX.1-schnell/resolve/main/flux1-schnell.safetensors",
SubDir: "unet",
File:   "flux1-schnell.safetensors",
},
{
Name:   "FLUX.1-dev (fp8)",
Short:  "High-quality 12B model — slower, requires HF token",
Size:   "~17 GB",
URL:    "https://huggingface.co/black-forest-labs/FLUX.1-dev/resolve/main/flux1-dev.safetensors",
SubDir: "unet",
File:   "flux1-dev.safetensors",
},
{
Name:   "Stable Diffusion XL base 1.0",
Short:  "Versatile 6.9 GB SDXL model",
Size:   "~6.9 GB",
URL:    "https://huggingface.co/stabilityai/stable-diffusion-xl-base-1.0/resolve/main/sd_xl_base_1.0.safetensors",
SubDir: "checkpoints",
File:   "sd_xl_base_1.0.safetensors",
},
{
Name:   "Stable Diffusion 3.5 Medium",
Short:  "Balanced 2.5B model — fast on Apple Silicon",
Size:   "~5.9 GB",
URL:    "https://huggingface.co/stabilityai/stable-diffusion-3.5-medium/resolve/main/sd3.5_medium.safetensors",
SubDir: "checkpoints",
File:   "sd3.5_medium.safetensors",
},
{
Name:   "CLIP-L (text encoder for FLUX/SD3)",
Short:  "Required text encoder for FLUX and SD3 workflows",
Size:   "~246 MB",
URL:    "https://huggingface.co/openai/clip-vit-large-patch14/resolve/main/model.safetensors",
SubDir: "clip",
File:   "clip_l.safetensors",
},
{
Name:   "T5-XXL fp8 (text encoder for FLUX)",
Short:  "Required text encoder for FLUX workflows",
Size:   "~4.9 GB",
URL:    "https://huggingface.co/comfyanonymous/flux_text_encoders/resolve/main/t5xxl_fp8_e4m3fn.safetensors",
SubDir: "clip",
File:   "t5xxl_fp8_e4m3fn.safetensors",
},
{
Name:   "FLUX VAE",
Short:  "Required VAE decoder for FLUX models",
Size:   "~335 MB",
URL:    "https://huggingface.co/black-forest-labs/FLUX.1-schnell/resolve/main/ae.safetensors",
SubDir: "vae",
File:   "flux_ae.safetensors",
},
}

func newModelsCmd(cfg *config.Config) *cobra.Command {
cmd := &cobra.Command{
Use:   "models",
Short: "List and download ComfyUI image-generation models",
RunE:  func(cmd *cobra.Command, args []string) error { return runModels(cfg) },
}
return cmd
}

func runModels(cfg *config.Config) error {
pterm.DefaultSection.WithLevel(2).Println("ComfyUI Model Manager")

// ── Show installed models ──────────────────────────────────────────────
showInstalled(cfg)

// ── Show catalog ──────────────────────────────────────────────────────
pterm.DefaultSection.WithLevel(3).Println("Available models")
tableData := pterm.TableData{{"#", "Name", "Size", "Directory", "Status"}}
for i, m := range catalog {
dst := filepath.Join(cfg.ComfyUIDir, "models", m.SubDir, m.File)
status := pterm.FgGray.Sprint("not installed")
if _, err := os.Stat(dst); err == nil {
status = pterm.FgGreen.Sprint("✓ installed")
}
tableData = append(tableData, []string{
fmt.Sprintf("%d", i+1),
m.Name,
m.Size,
m.SubDir + "/" + m.File,
status,
})
}
_ = pterm.DefaultTable.WithHasHeader(true).WithData(tableData).Render()

fmt.Println()
pterm.Info.Println("Tip: FLUX.1-schnell needs clip_l + t5xxl_fp8 + flux_ae to work.")
pterm.Info.Println("     SD XL / SD 3.5 Medium work out-of-the-box (no extra files).")
fmt.Println()

// ── Interactive download ───────────────────────────────────────────────
choices := make([]string, 0, len(catalog)+1)
for i, m := range catalog {
dst := filepath.Join(cfg.ComfyUIDir, "models", m.SubDir, m.File)
label := fmt.Sprintf("[%d] %s (%s)", i+1, m.Name, m.Size)
if _, err := os.Stat(dst); err == nil {
label += "  ✓"
}
choices = append(choices, label)
}

selected, err := pterm.DefaultInteractiveMultiselect.
WithOptions(choices).
WithDefaultText("Select models to download (Space to toggle, Enter to confirm)").
Show()
if err != nil || len(selected) == 0 {
pterm.Info.Println("Nothing selected — done.")
return nil
}

// Map selected labels back to catalog entries
toDownload := []modelEntry{}
for _, sel := range selected {
for i, ch := range choices {
if sel == ch {
toDownload = append(toDownload, catalog[i])
break
}
}
}

fmt.Println()
for _, m := range toDownload {
if err := downloadModel(cfg, m); err != nil {
pterm.Error.Printf("Failed: %s — %v\n", m.Name, err)
}
}

fmt.Println()
pterm.Success.Println("Done. Restart ComfyUI to load new models: llamactl comfyui restart")
return nil
}

// showInstalled prints a summary of already-downloaded model files.
func showInstalled(cfg *config.Config) {
pterm.DefaultSection.WithLevel(3).Println("Installed models")
dirs := []string{"checkpoints", "unet", "diffusion_models", "loras", "vae", "clip"}
found := false
for _, d := range dirs {
entries, err := os.ReadDir(filepath.Join(cfg.ComfyUIDir, "models", d))
if err != nil {
continue
}
for _, e := range entries {
if !e.IsDir() && e.Name() != "put_checkpoints_here" && e.Name() != "put_diffusion_model_files_here" {
info, _ := e.Info()
size := ""
if info != nil {
gb := float64(info.Size()) / 1e9
if gb >= 1 {
size = fmt.Sprintf("%.1f GB", gb)
} else {
size = fmt.Sprintf("%d MB", info.Size()/1e6)
}
}
pterm.Printf("  %-15s %s  (%s)\n", pterm.FgCyan.Sprint(d+"/"), e.Name(), size)
found = true
}
}
}
if !found {
pterm.Warning.Println("No models installed yet.")
}
fmt.Println()
}

// downloadModel downloads a model file using wget or curl with a progress bar.
func downloadModel(cfg *config.Config, m modelEntry) error {
dst := filepath.Join(cfg.ComfyUIDir, "models", m.SubDir, m.File)

// Check if already exists
if _, err := os.Stat(dst); err == nil {
pterm.Success.Printf("Already installed: %s\n", m.Name)
return nil
}

if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
return err
}

pterm.DefaultSection.WithLevel(3).Printf("Downloading %s (%s)", m.Name, m.Size)
pterm.Info.Printf("  → %s\n", dst)
fmt.Println()

// Prefer wget (shows resumable progress); fall back to curl
var cmd *exec.Cmd
if _, err := exec.LookPath("wget"); err == nil {
cmd = exec.Command("wget", "--continue", "--show-progress", "-O", dst, m.URL)
} else {
cmd = exec.Command("curl", "-L", "--progress-bar", "-C", "-", "-o", dst, m.URL)
}
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr

if err := cmd.Run(); err != nil {
// Clean up partial file on failure
_ = os.Remove(dst)
return fmt.Errorf("download failed: %w", err)
}

pterm.Success.Printf("Downloaded: %s\n\n", m.File)
return nil
}
