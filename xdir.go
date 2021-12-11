/*
A clone of the Windows's dir command with nearly the same command options.

Because the program does its own globbing it is better to turn shell globbing
off under Linux (otherwise you need to use quotes around wildcards).
In your .bashrc add:

reset_expansion(){ CMD="$1";shift;$CMD "$@";set +f;}
alias xdir='set -f;reset_expansion $GOBIN/xdir'

stackoverflow.com/questions/11456403/stop-shell-wildcard-character-expansion
*/

package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"strings"
	"strconv"
	"time"
	"path/filepath"
	"github.com/tsaost/util"
	"github.com/tsaost/util/cmd"
	"github.com/tsaost/util/format"
)

var isSortByTime, isSortByTimeReversed bool
var isSortBySize, isSortBySizeReversed bool
var isSortByName, isSortByNameReversed bool
var isSortByExtension, isSortByExtensionReversed bool

var isShowHiddenFilesOnly, isExcludeHiddenFiles, isHiddenOptionExplicit bool
var isShowReadOnlyFilesOnly, isExcludeReadOnlyFiles bool
var isShowDirectoryOnly, isExcludeDirectory bool
var isRecurseSubDirectory, isNoCommaSeparator bool
var isShowFullPath, isShowPartialPath bool
var isBareDisplayFormat, isWideDisplayFormat, isMatchAllFiles bool
var isShowVolumeInformation, isUnixStyleListing, isShowNumericUnixFileMode bool
var isShowQuoteForFileWithSpaces bool
var numberOfHeadLines, numberOfTailLines int

var isWindows = runtime.GOOS == "windows"
var isUnix = !isWindows
var isIgnoreFilenameCase bool
var isSortByDirThenName = true
var wideFormatLineWidth = 80
var fileCutoffTime = time.Time{}

type byDirectoryThenName []util.PathInfo
func (f byDirectoryThenName) Len() int           { return len(f) }
func (f byDirectoryThenName) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f byDirectoryThenName) Less(i, j int) bool {
	iIsDir := f[i].IsDir()
	jIsDir := f[j].IsDir()
	if iIsDir && !jIsDir {
		return true
	}
	if !iIsDir && jIsDir {
		return false
	}
	return strings.ToLower(f[i].Name()) < strings.ToLower(f[j].Name())
}

type byName []util.PathInfo
func (f byName) Len() int           { return len(f) }
func (f byName) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f byName) Less(i, j int) bool {
	return strings.ToLower(f[i].Name()) < strings.ToLower(f[j].Name())
}

type byNameReversed []util.PathInfo
func (f byNameReversed) Len() int           { return len(f) }
func (f byNameReversed) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f byNameReversed) Less(i, j int) bool {
	return strings.ToLower(f[i].Name()) > strings.ToLower(f[j].Name())
}
	
type byExtension []util.PathInfo
func (f byExtension) Len() int           { return len(f) }
func (f byExtension) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f byExtension) Less(i, j int) bool {
	return strings.ToLower(filepath.Ext(f[i].Name())) <
		strings.ToLower(filepath.Ext(f[j].Name()))
}

type byExtensionReversed []util.PathInfo
func (f byExtensionReversed) Len() int           { return len(f) }
func (f byExtensionReversed) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f byExtensionReversed) Less(i, j int) bool {
	return strings.ToLower(filepath.Ext(f[i].Name())) >
		strings.ToLower(filepath.Ext(f[j].Name()))
}
	
type byTime []util.PathInfo
func (f byTime) Len() int { return len(f) }
func (f byTime) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f byTime) Less(i, j int) bool { 
	return f[i].ModTime().Before(f[j].ModTime())
}

type byTimeReversed []util.PathInfo
func (f byTimeReversed) Len() int           { return len(f) }
func (f byTimeReversed) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f byTimeReversed) Less(i, j int) bool {
	return f[i].ModTime().After(f[j].ModTime())
}

type bySize []util.PathInfo
func (f bySize) Len() int           { return len(f) }
func (f bySize) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f bySize) Less(i, j int) bool { return f[i].Size() < f[j].Size() }

type bySizeReversed []util.PathInfo
func (f bySizeReversed) Len() int           { return len(f) }
func (f bySizeReversed) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f bySizeReversed) Less(i, j int) bool { return f[i].Name() > f[j].Name() }

var totalFilesCount, totalDirectoriesCount int
var totalFilesSize int64

func getWideFormatFileListing(infos []util.PathInfo) []string {
	listing := make([]string, 0, len(infos))

    maxLen := 13
    for _, x := range infos {
		name := x.Name()
		lenName := len(name)
		if x.IsDir() {
			lenName += 2 // Need to put [..] around directory 
		}
		if lenName > maxLen {
			maxLen = lenName
		}
	}

	const spaces = "                                                          "
	const maxSpacesLen = len(spaces)

	maxLen++ // Need at least one space between names
	if maxLen > maxSpacesLen {
		maxLen = maxSpacesLen
	}
	entriesPerLine := wideFormatLineWidth / maxLen
	if entriesPerLine == 0 {
		entriesPerLine = 1
	}
	i, line := 0, ""
    for _, x := range infos {
		name := x.Name()
		if x.IsDir() {
			name = "[" + name + "]"
		}
		line = line + name
		i++
		if i == entriesPerLine {
			listing = append(listing, line)
			i, line = 0, ""
		} else {
			spacing := maxLen - len(name)
			if spacing < 0 {
				spacing = 0
			} else if spacing > maxSpacesLen {
				spacing = maxSpacesLen
			}
			line += spaces[:spacing]
		}
	}
	if i > 0 {
		listing = append(listing, line)
	}
	return listing
}


func getWindowsLongFileListing(infos []util.PathInfo, sizeWidth int) []string {
	listing := make([]string, len(infos), len(infos))
	listingFormat := "%04d-%02d-%02d  %02d:%02d %s  %" +
		strconv.Itoa(sizeWidth) +"s %s"
	for i, info := range(infos) {
		isSymlink := info.Mode() & os.ModeSymlink == os.ModeSymlink
		name := info.Name()
		pathName := info.PathName()
		isDir := info.IsDir()
		var linkTarget string
		if isSymlink {
			// fmt.Println(pathName)
			// if link, err := filepath.EvalSymlinks(pathName); err != nil {
			if link, err := util.Readlink(pathName); err == nil {
				linkTarget = " [" + link + "]"
				if !isDir {
					// hack hack hack
					// treat links to directories as directory so <JUNCTION>
					// will appear in the listing just as under Windows
					var targetInfo os.FileInfo
					if targetInfo, err = os.Stat(link); err == nil {
						isDir = targetInfo.IsDir()
					}
				}
			}
		}

		if isBareDisplayFormat {
			displayName := pathName
			if !isShowFullPath &&
				strings.HasPrefix(pathName, currentWorkingDirectory) {
				displayName = displayName[displayPathStart:]
			}
			if info.IsDir() && !isShowDirectoryOnly {
				displayName = "[" + displayName + "]"
			}
			if linkTarget != "" {
				displayName += linkTarget
			}
			if isShowQuoteForFileWithSpaces &&
				strings.Contains(displayName, " ") {
				displayName = "\"" + displayName + "\""
			}
			listing[i] = displayName
			continue
		}

		var size string
		if isDir {
			totalDirectoriesCount++
			if isSymlink {
				size = "<JUNCTION>    "
			} else {
				size = "<DIR>         "
			}
		} else {
			size = format.CommaSeparated(info.Size())
		}

		t := info.ModTime().Local()
		hour, amPM := t.Hour(), "AM"
		if hour > 12 {
			hour, amPM = hour - 12, "PM"
		}
		displayName := pathName
		if isShowPartialPath {
			if strings.HasPrefix(pathName, currentWorkingDirectory) {
				displayName = pathName[displayPathStart:]
			} 
		} else if !isShowFullPath {
			displayName = name
		}
		listing[i] = fmt.Sprintf(listingFormat,
			t.Year(), t.Month(), t.Day(), hour, t.Minute(), amPM,
			size, displayName + linkTarget)
	}
	return listing
}


func printDirectoryListing(listing []string) {
	const omitted = "..........  ..... .."
	if numberOfHeadLines != 0 && numberOfHeadLines < len(listing) {
		listing = append(listing[:numberOfHeadLines], omitted)
	} else if numberOfTailLines != 0 && numberOfTailLines < len(listing) {
		listing = listing[len(listing) - numberOfTailLines - 1:]
		listing[0] = omitted
	}

	for _, line := range listing {
		fmt.Println(line)
	}
}


func showDirectoryListing(directory string, args []string) error {
	// fmt.Println("directory:", directory)
	f, err := os.Open(directory)
	if err != nil {
		return err
	}

	// ioutil.ReadDir() is not used because we don't need the names to be sorted
	allInfos, err := f.Readdir(-1); f.Close()
	if err != nil {
		return err
	}

	subDirectories := []string{}
	infos := make([]util.PathInfo, 0, len(allInfos))

	var totalSizes, maxSize int64
	filesCount, directoriesCount, maxNameLen := 0, 0, 0
	for _, x := range allInfos {
		name := x.Name()
		if name == "." || name == ".." {
			continue
		}
		pathName := filepath.Join(directory, name)
		isDir := x.IsDir() 
		if isDir {
			if isRecurseSubDirectory {
				subDirectories = append(subDirectories, pathName)
			}
			if isExcludeDirectory {
				continue
			}
		} else if isShowDirectoryOnly {
			continue
		}

		if x.ModTime().Before(fileCutoffTime) {
			continue
			// && !(isDir && isMatchAllFiles) {
			// cutoff does not exclude directories unless match is specified
		}

		matched := isMatchAllFiles
		if !matched {
		    target := name
		    if isIgnoreFilenameCase {
				target = strings.ToLower(name)
			}
			for _, y := range(args) {
				if matched, err = filepath.Match(y, target); err != nil {
					return err
				} 
				if matched {
					break
				}
			}
		}

		if matched {
			if isExcludeHiddenFiles || isShowHiddenFilesOnly {
				if hidden, err1 := util.IsHiddenFile(pathName, true); err1!=nil{
					// the error "The filename, directory name,
					// or volume label syntax is incorrect." will occur
					// if the directory is somehow corrupted, but want to
					// continue anyway
					fmt.Printf("Warning \"%s\": %v", pathName, err1)
					continue
				} else if hidden {
					// fmt.Println("hidden:", name)
					if isExcludeHiddenFiles && !(isDir && isShowDirectoryOnly) {
						// Follow "dir /ad" to show hidden directories
						continue
					}
				} else {
					// hack hack hack: also show system files
					if system, err2 := util.IsSystemFile(pathName); err2 != nil{
						return err2
					} else if system && !(isDir && isShowDirectoryOnly) {
						fmt.Println("system:", name)
						// Follow "dir /ah " to show system directories
						if isExcludeHiddenFiles {
							continue
						}
					}
					if isShowHiddenFilesOnly {
						continue
					}
				}
			}
			if isExcludeReadOnlyFiles || isShowReadOnlyFilesOnly {
				if readonly, err1 := util.IsReadOnlyFile(pathName); err1 != nil{
					// the error "The filename, directory name,
					// or volume label syntax is incorrect." will occur
					// if the directory is somehow corrupted, but want to
					// continue anyway
					fmt.Printf("Warning \"%s\": %v", pathName, err1)
					continue
				} else if readonly {
					// fmt.Println("hidden:", name)
					if isExcludeReadOnlyFiles {
						continue
					}
				} else if isShowReadOnlyFilesOnly {
					continue
				}
			}

			infos = append(infos, util.NewPathInfo(x, pathName))
			if isDir {
				directoriesCount++
			} else {
				filesCount++
				size := x.Size()
				totalSizes += size
				if size > maxSize {
					maxSize = size
				}
			}
			if len(name) > maxNameLen {
				maxNameLen = len(name) 
			}
		}
	}		

	if isSortByTime {
		sort.Sort(byTime(infos))
	} else if isSortBySize {
		sort.Sort(bySize(infos))
	} else if isSortByExtension {
		sort.Sort(byExtension(infos))
	} else if isSortByTimeReversed {
		sort.Sort(byTimeReversed(infos))
	} else if isSortBySizeReversed {
		sort.Sort(bySizeReversed(infos))
	} else if isSortByExtensionReversed {
		sort.Sort(byExtensionReversed(infos))
	} else if isSortByName {
		sort.Sort(byName(infos))
	} else if isSortByNameReversed {
		sort.Sort(byNameReversed(infos))
	} else if isSortByDirThenName {
		sort.Sort(byDirectoryThenName(infos))
	}

	var listing []string
	if isWideDisplayFormat {
		listing = getWideFormatFileListing(infos)
	} else if isUnixStyleListing {
		listing = cmd.GetUnixLongFileListing(infos, isShowFullPath,
			isShowPartialPath, isShowNumericUnixFileMode,
			currentWorkingDirectory, displayPathStart)
	} else {
		sizeFieldWidth := cmd.MaxFileSizeWidth
		if maxNameLen > wideFormatLineWidth - (cmd.MaxFileSizeWidth+20) {
			// Try to use cmd.MaxFileSizeWidth, unless maxNameLen is larger
			// than the available width 
			sizeFieldWidth = len(format.CommaSeparated(maxSize))
		}
		listing = getWindowsLongFileListing(infos, sizeFieldWidth)
	}

	printDirectoryListing(listing)

	if (filesCount > 0 || directoriesCount > 0) /*&& !isBareDisplayFormat*/ {
		relativeDirectory := directory
		if strings.HasPrefix(directory,filepath.Dir(currentWorkingDirectory)) &&
			len(directory) > displayDirStart {
			relativeDirectory = directory[displayDirStart:]
			if strings.IndexByte(relativeDirectory, os.PathSeparator) < 0 {
				relativeDirectory = fmt.Sprintf(".%c%s", os.PathSeparator,
					relativeDirectory)
			}
		}
		fmt.Println()
		if filesCount == 1 && !isBareDisplayFormat {
			fmt.Printf("%" + strconv.Itoa(cmd.MaxFileSizeWidth + 5) +
				"s Only one file in %s\n", "", relativeDirectory)
		} else if filesCount > 1 {
			fmt.Printf("%" + strconv.Itoa(cmd.MaxFileSizeWidth + 22) + 
				"s %s\n", fmt.Sprintf("%d Files %s (%d bytes)", filesCount, 
					format.CommaSeparated(totalSizes), totalSizes),
				relativeDirectory)
		} else if directoriesCount == 1 {
			fmt.Printf("%" + cmd.MaxFileSizeWidthText +
				"s Only one directory in %s\n", "", relativeDirectory)
		} else if directoriesCount > 1 {
			fmt.Printf("%" + cmd.MaxFileSizeWidthText +
				"d directories in %s\n",
				directoriesCount, relativeDirectory)
		}
		if isRecurseSubDirectory {
			fmt.Println()
		}
	}

	for _, x := range subDirectories {
		if err = showDirectoryListing(x, args); err != nil {
			// return err
			fmt.Println(err)
			continue
		}
	}

	totalDirectoriesCount += directoriesCount
	totalFilesCount += filesCount
	totalFilesSize += totalSizes
	return nil
}


func showAbsPathList(pathList []string) error {
	// because the paths could be anywhere in the system, must show at least
	// part of the path to distiguish between dir1/abc and dir2/abc
	// fmt.Printf("%t showAbsPathList: %v\n", isShowFullPath, pathList)
	isShowPartialPath = !isShowFullPath

	totalFilesCount, totalFilesSize = 0, 0
	infos := make([]util.PathInfo, 0, len(pathList))
	for _, pathName := range pathList {
		info, err := os.Lstat(pathName)
		if err != nil {
			// fmt.Printf("%s: %s\n", pathName, err)
			fmt.Printf("%s\n", err)
		} else {
			infos = append(infos, util.NewPathInfo(info, pathName))
			totalFilesCount++
			totalFilesSize += info.Size()
		}
	}

	var listing []string
	if isUnixStyleListing {
		listing = cmd.GetUnixLongFileListing(infos, isShowFullPath,
			isShowPartialPath, isShowNumericUnixFileMode,
			currentWorkingDirectory, displayPathStart)
	} else {
		listing = getWindowsLongFileListing(infos, cmd.MaxFileSizeWidth)
	}
	printDirectoryListing(listing)
	fmt.Println()

	isMatchAllFiles = true 
	isExcludeHiddenFiles = true
	isShowPartialPath = false
	args := []string{}
	for _, x := range infos {
		// fmt.Println("x.IsDir()", x.IsDir())
		if x.IsDir() {
			showDirectoryListing(x.PathName(), args)
			fmt.Println()
		}
	}

	return nil
}


var isOptionMustStartWithMinus bool

func parseOneOption(arg string) (bool, string) {
	ch := arg[0]
	if ch == 'd' || ch == 'h' || ch == 't' || ch == 'w' {
		value, restOfArg := cmd.ParseNumericArg(arg, 0)
		switch ch {
		case 'd':
			if value == 0 {
				value = 1
			}
			fileCutoffTime = time.Now().
				Add(-time.Duration(value * 24) * time.Hour)
		case 'h':
			numberOfHeadLines = value
			if numberOfHeadLines == 0 {
				numberOfHeadLines = 25
			} 
		case 't':
			numberOfTailLines = value
			if numberOfTailLines == 0 {
				numberOfTailLines = 25
			}
		case 'w':
			if value > 0 {
				wideFormatLineWidth = value
			}
			isWideDisplayFormat = true
			
		default:
			panic("Unknown ch(" + arg[:1] + ")")
		} 
		return true, restOfArg
	}


	returnIndex := 1
	switch ch {
	case '?': usage(); os.Exit(1)
	case 'q': isShowQuoteForFileWithSpaces = true
		isBareDisplayFormat = true
	case 'b': isBareDisplayFormat = true
	case 'f': isShowFullPath = true
	case 'v': isShowVolumeInformation = true
	case 'z', 's', 'r':
		isRecurseSubDirectory = true
		if ch == 'z' {
			isShowFullPath = true
		}
		if isBareDisplayFormat {
			isExcludeDirectory = !isShowDirectoryOnly
		}

	case '-':
		returnIndex++
		if strings.HasPrefix(arg, "-c") {
			isNoCommaSeparator = true
		} else {
			return false, arg
		}

	case 'a':
		returnIndex++
		if strings.HasPrefix(arg, "ad") {
			if isExcludeDirectory {
				log.Fatal("Can not use both /ad and /a-d")
			}
			isShowDirectoryOnly = true
		} else if strings.HasPrefix(arg, "ah") || strings.HasPrefix(arg, "as") {
			isHiddenOptionExplicit = true
			isShowHiddenFilesOnly = true
			isExcludeHiddenFiles = false
		} else if strings.HasPrefix(arg, "ao") {
			isShowReadOnlyFilesOnly = true
			isExcludeReadOnlyFiles = false
		} else {
			returnIndex++
			if strings.HasPrefix(arg, "a-d") {
				if isShowDirectoryOnly {
					log.Fatal("Can not use both /a-d and /ad")
				}
				isExcludeDirectory = true
			} else if strings.HasPrefix(arg, "a-h") ||
				strings.HasPrefix(arg, "a-s") {
				isHiddenOptionExplicit = true
				if isShowHiddenFilesOnly {
					log.Fatal("Can not use both /a-h and /ah")
				}
				isExcludeHiddenFiles = true
			} else if strings.HasPrefix(arg, "a-o") {
				if isShowReadOnlyFilesOnly {
					log.Fatal("Can not use both /a-o and /ao")
				}
				isExcludeReadOnlyFiles = true
			} else {
				return false, arg
			}
		}

	case 'o':
		returnIndex++
		if strings.HasPrefix(arg, "on") {
			isSortByName = true; isSortByDirThenName = false
		} else if strings.HasPrefix(arg, "og") {
			isSortByName = true; isSortByDirThenName = true
		} else if strings.HasPrefix(arg, "os") {
			isSortBySize = true
		} else if strings.HasPrefix(arg, "od") {
			isSortByTime = true
		} else if strings.HasPrefix(arg, "oe") {
			isSortByExtension = true
		} else {
			returnIndex++
			if strings.HasPrefix(arg, "o-n") {
				isSortByNameReversed = true
			} else if strings.HasPrefix(arg, "o-s") {
				isSortBySizeReversed = true
			} else if strings.HasPrefix(arg, "o-d") {
				isSortByTimeReversed = true
			} else if strings.HasPrefix(arg, "o-e") {
				isSortByExtensionReversed = true
			} else {
				return false, arg
			}
		}

	case 'u':
		if isUnix {
			isUnixStyleListing = true
			break
		}
		return false, arg

	case 'x':
		if isUnix {
			isUnixStyleListing = true
			isShowNumericUnixFileMode = true
			break
		}
		return false, arg

	default:
		return false, arg
	}
	return true, arg[returnIndex:]
}

const optionEnvironmentVariable = "XDIROPTION"
const caseSensitivityEnvironmentVariable = "XDIRCASESENSITIVE"

func usage() {
	xdir := filepath.Base(os.Args[0])
	fmt.Printf("%s /[options] pattern1 pattern2 ....\n" +
        "    /w(wide) /b(are) /f(ullpath) \n" +
        "    /s(ubdirectory) or /r   search in subdirectories\n" +
        "    /z                      Like /s but show full path\n" +
        "    /h(head)[0-9]+          Show first few lines of listing\n" +
        "    /t(ail)[0-9]+           Show last few lines of listing\n" +
        "    /d(ays)[0-9]+           Show files no older than x days\n" +
        "    /on /od /os /oe /og     Sort by name, date, size, ext, dir\n" +
        "    /ad /a-d                Only show directory (- to exclude)\n" +
        "    /ah /a-h                Only show hidden/system (- to exclude)\n" +
        "    /as /a-s                Same as /ah /a-h\n" +
        "    /ao /a-o                Only show read-only (- to exclude)\n" +
		"    /q						 Quote filename with space (implies /b)\n" +
        "    /v                      Show volume info\n", xdir)
	if isUnix {
		fmt.Printf("     /u(nix style)           Unix style listing\n")
		fmt.Printf("     /x(xx Unix file mode)   Unix numeric mode listing \n")
	}
	fmt.Printf("\n" +
		"Unlike the original Windows dir command, hidden files \n" +
		"are shown by default if a glob pattern is specified.\n" +
		"To show all files including hidden ones, use *.* instead of *.\n" +
		"Use the /a-h option to override this default behavoir.\n\n")
	fmt.Printf("Options can start with '-' instead of '/', " +
		"but don't mix them. For example:\n" + 
		"     %s /h10t15osbs d:\\workspace\\go\\src*.go *.txt\n" +
		"     %s -h10t15osbs ~/workspace/go/src*.go *.txt\n\n", xdir, xdir)
	cmd.PrintUsageOptionEnvironmentVariables(xdir, optionEnvironmentVariable,
		caseSensitivityEnvironmentVariable)
}

var currentWorkingDirectory string
var displayPathStart, displayDirStart int
var startDirectory string

func parseArgAsOptions(arg string) bool {
	return cmd.ParseCommandLineOptions(arg, &isOptionMustStartWithMinus,
		parseOneOption)
}

func main() {
	log.SetFlags(0)
	// wideFormatLineWidth = cmd.InitializeConsoleScreenWidth()
	wideFormatLineWidth = cmd.GetConsoleScreenWidth()
	isIgnoreFilenameCase =
		!cmd.IsFileNameCaseSensitive(caseSensitivityEnvironmentVariable)
	wideFormatLineWidth = 80

	if options := os.Getenv(optionEnvironmentVariable); len(options) > 0 {
		for _, x := range strings.Split(options, " ") {
			if !parseArgAsOptions(x) {
				log.Fatal("Bad default option: ", x)
			}
		}
	}

	args := make([]string, 0, len(os.Args) - 1)
	for _, x := range os.Args[1:] {
		// fmt.Printf("x0(%s)\n", x)
		if !parseArgAsOptions(x) {
			// Don't do it here since it will mess up the src directory
			// which is case sentive under Linux
			// if isIgnoreFilenameCase {
			// 	    x = string.ToLower(x)
			// }
			args = append(args, x)
		}
	}

	currentWorkingDirectory, startDirectory,
	displayPathStart, displayDirStart, isMatchAllFiles, args =
		cmd.ExtractStartDirectory(args)
	// fmt.Println("startDirectory:", startDirectory, displayPathStart)

	diskVolumeName, diskSerialNumber, err := util.
		GetDiskVolumeNameSerialNumber(startDirectory)
	if err != nil {
		log.Fatal("GetDiskVolumeNameSerialNumber:", err)
	}

	// if diskVolumeName == "" {
	// 	diskVolumeName = "?????"
	// }

	if diskVolumeName != "" && !isBareDisplayFormat  && (isMatchAllFiles ||
		isShowVolumeInformation) {
		fmt.Printf("Volume in drive %s is %s, Serial %04X-%04X\n",
			strings.ToUpper(startDirectory[:2]), diskVolumeName,
			diskSerialNumber >> 16, diskSerialNumber & 0xffff)
	}

	absArgs := cmd.GetAbsPathListIfNoWildcardFound(startDirectory, args,
		isRecurseSubDirectory)
	// fmt.Println("absArgs:", absArgs, startDirectory, isMatchAllFiles)
	if len(absArgs) == 0 {
		if len(args) > 0 && args[0] == "" {
			args = args[1:]
		}
		if isIgnoreFilenameCase {
			for i, arg := range args {
				args[i] = strings.ToLower(arg)
			}
		}
		if isMatchAllFiles {
			if len(args) > 1 {
				log.Fatalf("You can not specify multiple pattern %v " +
					"in combination with * or *.*", args)
			}
			if !isHiddenOptionExplicit && len(args) == 0 {
				// By default, do not show hidden and system files if
				// trying to show everything, unless the option was set
				// explicityly via /ah or /a-h
				//
				// But if the match was to "*.*" or "*" then show them
				isExcludeHiddenFiles = true
			}
		}
		err = showDirectoryListing(startDirectory, args)
	} else {
		err = showAbsPathList(absArgs)
	} 
	if err != nil {
		log.Fatal(err)
	} 

    if totalFilesCount == 0 && totalDirectoriesCount == 0 {
		fmt.Printf("No file found\n")
	} else if totalFilesCount > 1 &&
		(isRecurseSubDirectory || len(absArgs) > 0) {
		fmt.Printf("%5d File(s)  %" + cmd.MaxFileSizeWidthText + "s bytes " +
			"total\n", totalFilesCount, format.CommaSeparated(totalFilesSize))
	}

	if isShowVolumeInformation || !isBareDisplayFormat {
		du, err := util.NewDiskUsage(startDirectory)
		if err != nil {
			log.Fatal("NewDiskUsage: ", err)
		}
		freeSpace := format.CommaSeparated(du.Free)
		if isWindows {
			if diskVolumeName == "" {
				diskVolumeName = "<Unknown>"
			}
			fmt.Printf("%" + strconv.Itoa(cmd.MaxFileSizeWidth + 16) +
				"s bytes free in %s (%s, %04X-%04X)\n", freeSpace,
				strings.ToUpper(startDirectory[:2]), diskVolumeName,
				diskSerialNumber >> 16, diskSerialNumber & 0xffff)
		} else if len(diskVolumeName) > 0 {
			fmt.Printf("%32s bytes free in volume %s\n",
				freeSpace, diskVolumeName)
		} else {
			fmt.Printf("%32s bytes free\n", freeSpace)
		}
	}
}
