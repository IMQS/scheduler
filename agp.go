package scheduler

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const agpFileContent = `<?xml version="1.0" encoding="utf-8" standalone="yes" ?>
<AlbionGisProject Version="1001" Path="C:\imqsvar\staging\za.--.--.--_Generic_SHAPEFILENAME\Optional#SHAPEFILENAME.agp">
  <Drawing Path="" PrevPath="" LoadDiffPathWarningConfirmed="1" SaveWithProject="1" />
  <CoordinateSystem Name="WGS_1984_Web_Mercator_Auxiliary_Sphere" CoordinateType="Cartesian" CoordinateUnit="Meters" PrimeMeridianName="" PrimeMeridianLon0="0">
    <Projection Name="Mercator">
      <Parameters Lat0="0" Lon0="0" FalseEasting="0" FalseNorthing="0" ScaleFactor="1" />
    </Projection>
    <Ellipsoid Name="sphere_6378137" />
  </CoordinateSystem>
  <Settings>
    <EnableIntelligentLabels val="1" />
    <SuppressIntelligentLabelsAtLoad val="0" />
  </Settings>
  <GisLayer visible="1" locked="0" folded="0">
    <GisLayerName value="SHAPEFILENAME" />
    <Data LoadDiffPathWarningConfirmed="0">
      <Database Connector="Shapefile">
        <File RelativePath="SHAPEFILENAME.shp" />
      </Database>
      <Table Name="Table" ModStamp="3DAD4774-F645-4F38-00F5-C39C1F7EBE77" />
    </Data>
    <GisLayerConfig>
      <RenderSettings>
        <Type1 Version="2">
          <Categorized Palette="Sequential-7-YlGn">
            <Default Label="&lt;all other values&gt;">
              <Output>
                <Fill Color="0.851 0.941 0.639 0.498" />
                <Stroke Color="0.678 0.753 0.51 1" />
                <Labels />
              </Output>
            </Default>
          </Categorized>
        </Type1>
      </RenderSettings>
    </GisLayerConfig>
  </GisLayer>
</AlbionGisProject>`

// CreateAGP takes the static content in "agpFileContent", adds the correct filename and then
// writes the data to the file in the matching folder
func CreateAGP(fileName string) bool {
	// Remove the extension from the fileName
	fileName = strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// Replace the SHAPEFILENAME place holder with the supplied one
	r := regexp.MustCompile("SHAPEFILENAME")
	agpFile := r.ReplaceAllString(agpFileContent, fileName)

	// Create the file
	file, err := os.Create("C:/imqsvar/staging/za.--.--.--_Generic_" + fileName + "/Optional#" + fileName + ".agp")
	if err != nil {
		return false
	}
	defer file.Close()

	// Write the modified AGP contents to the file
	nbytes, err := file.WriteString(agpFile)
	if err != nil || nbytes == 0 {
		return false
	}
	file.Sync()
	return true
}
