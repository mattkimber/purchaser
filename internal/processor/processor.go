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
)

const MAX_SIZE = 64
const START_X = 756
const END_X = 818

type Unit struct {
	ID string
	Cars int
	NextSprite string
	RequiresSecondPowerCar bool
	DoubleHeaded bool
	ReuseSpritesFrom string
	Template string
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

func processUnit(unit Unit) {

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

	sprite := unit.ID
	if unit.ReuseSpritesFrom != "" {
		sprite = unit.ReuseSpritesFrom
	}

	spriteImg, err := getPNG(fmt.Sprintf("1x/%s_8bpp.png", sprite))
	if err != nil {
		log.Printf("Error processing %s: %v", unit.ID, err)
		return
	}

	outputImg := image.NewPaletted(image.Rectangle{Max: image.Point{X: MAX_SIZE, Y: 17}}, spriteImg.ColorModel().(color.Palette))

	curX := 2

	if unit.Cars <= 1 {
		len := 0
		splitArticLen := strings.Split(unit.ArticulatedLengths, ",")
		for _, articLen := range splitArticLen {
			if l, err := strconv.Atoi(articLen); err == nil {
				len += l * 4
			}
		}

		curX = (MAX_SIZE / 2) - (len / 2)
	}

	for x := START_X; x < END_X; x++ {
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

	if unit.NextSprite != "" {
		spriteImg, err = getPNG(fmt.Sprintf("1x/%s_8bpp.png", unit.NextSprite))
		if err != nil {
			log.Printf("Error processing %s: %v", unit.ID, err)
			return
		}

		for x := START_X; x < END_X; x++ {
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
	}

	if overlay != "" {
		spriteImg, err = getPNG(overlay)
		if err != nil {
			log.Printf("Error processing %s: %v", unit.ID, err)
			return
		}

		curY := 17 - 1 - spriteImg.Bounds().Max.Y
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

	err = writePNG(unit.ID, outputImg)
	if err != nil {
		log.Printf("could not write image for unit %s: %v", unit.ID, err)
	}
}

func writePNG(id string, img image.PalettedImage) error {
	file, err := os.Create(fmt.Sprintf("1x/%s_purchase.png", id))
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

	nextSprite := ""
	if templateData["tender"] != "" {
		nextSprite = templateData["tender"]
	} else if templateData["layout"] != "" {
		layoutSplit := strings.Split(templateData["layout"], ",")
		if len(layoutSplit) > 1 {
			nextSprite = layoutSplit[1]
		}
	}

	unit := Unit{
		ID:                     templateData["id"],
		NextSprite:             nextSprite,
		RequiresSecondPowerCar: false,
		DoubleHeaded:           false,
		ReuseSpritesFrom:       templateData["reuse_sprites"],
		Template:			    templateData["template"],
		ArticulatedLengths:     templateData["articulated_lengths"],
	}

	unit.Cars, _ = strconv.Atoi(templateData["cars"])
	if templateData["requires_second_power_car"] != "" {
		unit.RequiresSecondPowerCar = true
	}

	if templateData["double_headed"] != "" {
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
		if fields[i] == "cars" || fields[i] == "tender" || fields[i] == "requires_second_power_car" ||
			fields[i] == "double_headed" || fields[i] == "reuse_sprites" || fields[i] == "layout" ||
			fields[i] == "id"  || fields[i] == "template" || fields[i] == "articulated_lengths" {
			requiredFields++
		}

	}

	if requiredFields < 8 {
		log.Printf("CSV headers: %v", fields)
		err = fmt.Errorf("did not find template, id, cars, tender, reuse_sprites, requires_second_power_car, double_headed, articulated_lengths and layout columns in csv file")
		return
	}

	return
}