package ftpclient

import (
    "errors"
    "strings"
    "strconv"
    "time"
    "os"
    //"log"
)

// ftpFileInfo describes a file.
type ftpFileInfo struct {
    name        string
    size        int64
    mode        os.FileMode
    mtime       time.Time
    raw         string
}

func (f *ftpFileInfo) Name() string {
    return f.name
}

func (f *ftpFileInfo) Size() int64 {
    return f.size
}

func (f *ftpFileInfo) Mode() os.FileMode {
    return f.mode
}

func (f *ftpFileInfo) ModTime() time.Time {
    return f.mtime
}

func (f *ftpFileInfo) IsDir() bool {
    return f.mode.IsDir()
}

func (f *ftpFileInfo) Sys() interface{} {
    return f.raw
}

var errUnknownFormat = errors.New("Unknown format")

var formatParsers = []func(line string) (os.FileInfo, error){
    parseUnixFormat,
	parseDosFormat,
}

// Parse response string
func parse(line string) (os.FileInfo, error) {
    //log.Println(line)
	for _, f := range formatParsers {
		fileInfo, err := f(line)
		if err == errUnknownFormat {
			continue
		}
		return fileInfo, err
	}
	return nil, errUnknownFormat
}

func parseDosDateTime(input string) (dateTime time.Time, err error) {
    dateTime, err = time.Parse("01-02-06  03:04PM", input)
    if err == nil {
        return dateTime, err
    }
    
    dateTime, err = time.Parse("2006-01-02  15:04", input)
    return dateTime, err    
}

func parseDosFormat(input string) (os.FileInfo, error) {
    value := input[:17]
    mtime, err := parseDosDateTime(value)
	if err != nil {
		return nil, errUnknownFormat
	}

    var size uint64
    var mode os.FileMode
    
    value = input[17:]
	value = strings.TrimLeft(value, " ")
	if strings.HasPrefix(value, "<DIR>") {
        mode |= os.ModeDir
		value = strings.TrimPrefix(value, "<DIR>")
	} else {
		space := strings.Index(value, " ")
		if space == -1 {
    		return nil, errUnknownFormat
		}
		size, err = strconv.ParseUint(value[:space], 10, 64)
		if err != nil {
    		return nil, errUnknownFormat
		}
        
		value = value[space:]
	}

	name := strings.TrimLeft(value, " ")
	f := &ftpFileInfo{
        name: name,
        size: int64(size),
        mode: mode,
        mtime: mtime,
        raw: input,
    }
    
	return f, nil
}

func parseUnixFormat(input string) (os.FileInfo, error) {
    var err error
    var name string
    var size uint64
    var mode os.FileMode
    var mtime time.Time
    
	fields := strings.Fields(input)
	if len(fields) < 9 {
		return nil, errUnknownFormat
	}

    // type
	switch fields[0][0] {
	//case '-':
	case 'd':
        mode |= os.ModeDir
	case 'l':
        mode |= os.ModeSymlink
    case 'b':
        mode |= os.ModeDevice
	case 'c':
        mode |= os.ModeCharDevice
    case 'p', '=':
        mode |= os.ModeNamedPipe
    case 's':
        mode |= os.ModeSocket
	}

    // permission   
    for i := 0; i < 3; i++ {
        if fields[0][i*3+1] == 'r' {
            mode |= os.FileMode(04 << (3 * uint(2-i)))
        }
        if fields[0][i*3+2] == 'w' {
            mode |= os.FileMode(02 << (3 * uint(2-i)))
        }
        if fields[0][i*3+3] == 'x' || fields[0][i*3+3] == 's' {
            mode |= os.FileMode(01 << (3 * uint(2-i)))
        }
    }

    // size
    size, err = strconv.ParseUint(fields[4], 0, 64)
    if err != nil {
        return nil, err
    }

    // datetime
    mtime, err = parseDateTime(fields[5:8])
	if err != nil {
		return nil, err
	}

    // name
	name = strings.Join(fields[8:], " ")
    
	f := &ftpFileInfo{
        name: name,
        size: int64(size),
        mode: mode,
        mtime: mtime,
        raw: input,
    }
    
	return f, nil
}

func parseDateTime(fields []string) (mtime time.Time, err error) {
	var value string
	if strings.Contains(fields[2], ":") {
		thisYear, _, _ := time.Now().Date()
		value = fields[1] + " " + fields[0] + " " + strconv.Itoa(thisYear)[2:4] + " " + fields[2] + " GMT"
	} else {
		if len(fields[2]) != 4 {
			return mtime, errors.New("Invalid year format in time string")
		}
		value = fields[1] + " " + fields[0] + " " + fields[2][2:4] + " 00:00 GMT"
	}

	mtime, err = time.Parse("_2 Jan 06 15:04 MST", value)
    return
}
