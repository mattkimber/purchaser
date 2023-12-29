package processor

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const MAX_SIZE = 64
const START_X_LARGE = 757
const END_X_LARGE = 819

const START_X_SMALL = 180
const END_X_SMALL = 216

type Unit struct {
	ID                     string
	Cars                   int
	Sprites                []string
	RequiresSecondPowerCar bool
	DoubleHeaded           bool
	ReuseSpritesFrom       string
	Template               string
	Length                 int
	OverrideLengthPerUnit  []int
	ArticulatedLengths     string
}

func Process(filename string) error {
	csvFile, err := os.Open(filename)
	defer csvFile.Close()
	if err != nil {
		return err
	}

	rd := csv.NewReader(bufio.NewReader(csvFile))
	data, err := rd.ReadAll()
	if err != nil {
		return err
	}

	fields, err := getFields(data)
	if err != nil {
		return err
	}

	for _, d := range data[1:] {
		unit := getUnit(d, fields)
		processUnit(unit, 1)
		processUnit(unit, 2)
	}

	return nil
}

func fileIsNewerThanDate(filename string, date time.Time) (bool, error) {
	fileStats, err := os.Stat(filename)

	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if fileStats.ModTime().After(date) {
		return true, nil
	}

	return false, nil
}

func processUnit(unit Unit, scale int) {

	var newestInput time.Time

	if unit.Template == "na" || unit.Template == "tender" {
		return
	}
	overlay := ""

	if unit.Cars > 0 && (len(unit.OverrideLengthPerUnit) == 0 || (len(unit.OverrideLengthPerUnit) == 1 && unit.OverrideLengthPerUnit[0] == 0)) {
		overlay = fmt.Sprintf("purchase_sprites/x%d.png", unit.Cars)
	} else if unit.RequiresSecondPowerCar {
		overlay = "purchase_sprites/second_power_car.png"
	} else if unit.DoubleHeaded {
		overlay = "purchase_sprites/double_headed.png"
	}

	sprites := unit.Sprites
	if unit.ReuseSpritesFrom != "" {
		sprites[0] = unit.ReuseSpritesFrom
	}

	var outputImg *image.Paletted
	offset := 0
	curX := 2
	ySize := 17 * scale

	if unit.Cars <= 1 {
		len := 0
		splitArticLen := strings.Split(unit.ArticulatedLengths, ",")
		for _, articLen := range splitArticLen {
			if l, err := strconv.Atoi(articLen); err == nil {
				len += l * 4 * scale
			}
		}

		curX = ((MAX_SIZE * scale) / 2) - (len / 2)
		// If this breaks due to vehicle size, give up
		if curX < 1 {
			curX = 2
		}
		offset = curX
	}

	for idx, sprite := range sprites {
		filename := fmt.Sprintf("%dx/%s_8bpp.png", scale, sprite)
		purchaseOverlayFilename := fmt.Sprintf("%dx/%s_purchase_overlay_8bpp.png", scale, sprite)
		animFilename := fmt.Sprintf("%dx/%s_anim_1_8bpp.png", scale, sprite)
		pantographFilename := fmt.Sprintf("%dx/%s_pan_up_8bpp.png", scale, sprite)
		hasAnim := false

		// Identify the newest input file
		stats, err := os.Stat(filename)
		if err == nil {
			if stats.ModTime().After(newestInput) {
				newestInput = stats.ModTime()
			}
		}

		var animImage image.PalettedImage
		if _, err = os.Stat(purchaseOverlayFilename); err == nil {
			animImage, err = getPNG(purchaseOverlayFilename)
			if err == nil {
				hasAnim = true
			}
		} else if _, err = os.Stat(animFilename); err == nil && !hasAnim {
			animImage, err = getPNG(animFilename)
			if err == nil {
				hasAnim = true
			}
		} else if _, err = os.Stat(pantographFilename); err == nil && !hasAnim {
			animImage, err = getPNG(pantographFilename)
			if err == nil {
				hasAnim = true
			}
		}

		spriteImg, err := getPNG(filename)
		if err != nil {
			log.Printf("Error processing %s: %v", unit.ID, err)
			return
		}

		startX, endX := START_X_LARGE*scale, END_X_LARGE*scale
		if spriteImg.Bounds().Max.X < startX {
			startX, endX, ySize = START_X_SMALL*scale, END_X_SMALL*scale, 14*scale
		}

		if outputImg == nil {
			outputImg = image.NewPaletted(image.Rectangle{Max: image.Point{X: MAX_SIZE * scale, Y: ySize}}, spriteImg.ColorModel().(color.Palette))
		}

		for x := startX; x < endX; x++ {
			startDrawing := false

			for y := 0; y < spriteImg.Bounds().Max.Y; y++ {
				c := spriteImg.ColorIndexAt(x, y)
				if c != 0 {
					outputImg.SetColorIndex(curX, y, c)
				}
				if c != 0 && c != 255 {
					startDrawing = true
				}

				// Add the first anim frame for steamers, etc.
				if hasAnim {
					ac := animImage.ColorIndexAt(x, y)
					if ac != 0 && ac != 255 {
						outputImg.SetColorIndex(curX, y, ac)
					}
				}
			}

			if startDrawing {
				curX++
				if curX >= MAX_SIZE*scale {
					break
				}
			}
		}

		if idx == 0 && len(unit.OverrideLengthPerUnit) <= idx && curX > ((1+unit.Length+offset)*scale) {
			curX = 2 + ((1 + unit.Length + offset) * scale)
		} else if len(unit.OverrideLengthPerUnit) > idx && unit.OverrideLengthPerUnit[idx] != 0 {
			curX = (idx + 1) * unit.OverrideLengthPerUnit[idx] * scale
		}

		if curX >= MAX_SIZE*scale {
			break
		}
	}

	if overlay != "" {
		spriteImg, err := getPNG(overlay)
		if err != nil {
			log.Printf("Error processing %s: %v", unit.ID, err)
			return
		}

		curY := ySize - 1 - (spriteImg.Bounds().Max.Y * scale)
		curX = curX - 1 - (spriteImg.Bounds().Max.X * scale)

		for x := 0; x < spriteImg.Bounds().Max.X*scale; x++ {
			for y := 0; y < spriteImg.Bounds().Max.Y*scale; y++ {
				c := spriteImg.ColorIndexAt(x/scale, y/scale)
				if c != 0 {
					outputImg.SetColorIndex(curX+x, curY+y, c)
				}
			}
		}
	}

	err := writePNG(unit.ID, scale, outputImg, newestInput)
	if err != nil {
		log.Printf("could not write image for unit %s: %v", unit.ID, err)
	}
}

func writePNG(id string, scale int, img image.PalettedImage, newestInput time.Time) error {
	filename := fmt.Sprintf("%dx/%s_purchase.png", scale, id)

	newer, err := fileIsNewerThanDate(filename, newestInput)
	if newer || err != nil {
		return nil
	}

	file, err := os.Create(filename)
	defer file.Close()
	if err != nil {
		return err
	}

	return png.Encode(file, img)

}

func getPNG(sprite string) (image.PalettedImage, error) {
	file, err := os.Open(sprite)
	defer file.Close()
	if err != nil {
		return nil, err
	}

	img, err := png.Decode(file)
	return img.(image.PalettedImage), err
}

func getUnit(dataLine []string, fields []string) Unit {
	templateData := make(map[string]string)

	for i, f := range dataLine {
		templateData[fields[i]] = f
	}

	var sprites []string
	if hasTender, ok := templateData["tender"]; ok && hasTender != "" {
		sprites = []string{templateData["id"], templateData["tender"]}
	} else if templateData["layout"] != "" {
		sprites = strings.Split(templateData["layout"], ",")
	} else {
		sprites = []string{templateData["id"]}
	}

	articulatedLengths, _ := templateData["ttd_len"]
	if aLengths, ok := templateData["articulated_lengths"]; ok && aLengths != "" {
		articulatedLengths = aLengths
	}

	unit := Unit{
		ID:                     templateData["id"],
		Sprites:                sprites,
		RequiresSecondPowerCar: false,
		DoubleHeaded:           false,
		ReuseSpritesFrom:       templateData["reuse_sprites"],
		Template:               templateData["template"],
		ArticulatedLengths:     articulatedLengths,
	}

	unit.Cars, _ = strconv.Atoi(templateData["cars"])
	unit.Length, _ = strconv.Atoi(templateData["ttd_len"])
	splitOverride := strings.Split(templateData["purchase_length"], ",")

	for _, override := range splitOverride {
		overrideAsInt, _ := strconv.Atoi(override)
		unit.OverrideLengthPerUnit = append(unit.OverrideLengthPerUnit, overrideAsInt)
	}

	// convert to pixels
	unit.Length = unit.Length * 4

	if powerCar, ok := templateData["requires_second_power_car"]; ok && powerCar != "" {
		unit.RequiresSecondPowerCar = true
	}

	if dblHead, ok := templateData["double_headed"]; ok && dblHead != "" {
		unit.DoubleHeaded = true
	}

	return unit
}

func getFields(data [][]string) (fields []string, err error) {
	fields = make([]string, len(data[0]))

	requiredFields := 0

	for i, f := range data[0] {
		// CSVs found in the wild may have BOM in the header line
		fields[i] = strings.Trim(f, " \xEF\xBB\xBF")

		if fields[i] == "cars" || fields[i] == "layout" || fields[i] == "id" ||
			fields[i] == "template" || fields[i] == "ttd_len" {
			requiredFields++
		}

	}

	if requiredFields < 5 {
		log.Printf("CSV headers: %v", fields)
		err = fmt.Errorf("did not find template, id, cars, ttd_len, and layout columns in csv file")
		return
	}

	return
}
