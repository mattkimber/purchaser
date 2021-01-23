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
	ID string
	Cars int
	Sprites []string
	RequiresSecondPowerCar bool
	DoubleHeaded bool
	ReuseSpritesFrom string
	Template string
	Length int
	ArticulatedLengths string
}

func Process(filename string) error{
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
		processUnit(unit)
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

func processUnit(unit Unit) {

	var newestInput time.Time

	if unit.Template == "na" || unit.Template == "tender" {
		return
	}
	overlay := ""

	if unit.Cars > 0 {
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
	ySize := 17

	if unit.Cars <= 1 {
		len := 0
		splitArticLen := strings.Split(unit.ArticulatedLengths, ",")
		for _, articLen := range splitArticLen {
			if l, err := strconv.Atoi(articLen); err == nil {
				len += l * 4
			}
		}

		curX = (MAX_SIZE / 2) - (len / 2)
		offset = curX
	}

	for idx, sprite := range sprites {
		filename := fmt.Sprintf("1x/%s_8bpp.png", sprite)

		// Identify the newest input file
		stats, err := os.Stat(filename)
		if err == nil {
			if stats.ModTime().After(newestInput) {
				newestInput = stats.ModTime()
			}
		}

		spriteImg, err := getPNG(filename)
		if err != nil {
			log.Printf("Error processing %s: %v", unit.ID, err)
			return
		}

		startX, endX := START_X_LARGE, END_X_LARGE
		if spriteImg.Bounds().Max.X < startX {
			startX, endX, ySize = START_X_SMALL, END_X_SMALL, 14
		}

		if outputImg == nil {
			outputImg = image.NewPaletted(image.Rectangle{Max: image.Point{X: MAX_SIZE, Y: ySize}}, spriteImg.ColorModel().(color.Palette))
		}

		for x := startX; x < endX; x++ {
			startDrawing := false

			for y := 0; y < spriteImg.Bounds().Max.Y; y++ {
				c := spriteImg.ColorIndexAt(x, y)
				outputImg.SetColorIndex(curX, y, c)
				if c != 0 && c != 255 {
					startDrawing = true
				}
			}

			if startDrawing {
				curX++
				if curX >= MAX_SIZE {
					break
				}
			}
		}

		if idx == 0 && curX > (2 + 1 + unit.Length + offset) {
			curX = 2 + 1 + unit.Length + offset
		}

		if curX >= MAX_SIZE {
			break
		}
	}


	if overlay != "" {
		spriteImg, err := getPNG(overlay)
		if err != nil {
			log.Printf("Error processing %s: %v", unit.ID, err)
			return
		}

		curY := ySize - 1 - spriteImg.Bounds().Max.Y
		curX = curX - 1 - spriteImg.Bounds().Max.X

		for x := 0; x < spriteImg.Bounds().Max.X; x++ {
			for y := 0; y < spriteImg.Bounds().Max.Y; y++ {
				c := spriteImg.ColorIndexAt(x, y)
				if c != 0 {
					outputImg.SetColorIndex(curX+x, curY+y, c)
				}
			}
		}
	}

	err := writePNG(unit.ID, outputImg, newestInput)
	if err != nil {
		log.Printf("could not write image for unit %s: %v", unit.ID, err)
	}
}

func writePNG(id string, img image.PalettedImage, newestInput time.Time) error {
	filename := fmt.Sprintf("1x/%s_purchase.png", id)

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
		sprites = []string{ templateData["id"], templateData["tender"] }
	} else if templateData["layout"] != "" {
		sprites = strings.Split(templateData["layout"], ",")
	} else {
		sprites = []string { templateData["id"]}
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
		Template:			    templateData["template"],
		ArticulatedLengths:     articulatedLengths,
	}

	unit.Cars, _ = strconv.Atoi(templateData["cars"])
	unit.Length, _ = strconv.Atoi(templateData["ttd_len"])

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
			fields[i] == "template"  || fields[i] == "ttd_len" {
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