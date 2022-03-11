# How to Build the Tool

This utility is written in Go and uses the Fyne.io toolkit.  Therefore, at the very least you need a Go compiler installed, and then you need to install the Fyne.io toolkit, which itself has a few prerequisites.

**NOTE**:  My work is mostly done on macOS, but your setup should not be drastically different.

## Steps to Get Development Environment Setup

This is a bit of a journey, but I hope you enjoy the ride.

1. Go to https://go.dev/dl/ and download the installer for your OS.  
Pay attention that the installers shown in the boxes near the top are the most common ones (e.g., the Intel x64 version for MS Windows, macOS, and Linux).  So if you have an ARM64-based system, look down a bit for the installers specific to your OS/architecture.  (I note this mostly for macOS users who have Apple Silicon-based Macs, as otherwise by default you are installing and running the Intel x64 version under Rosetta vs. using the native version for your hardware.)
2. Install the Go compiler.  
This may also require you to adjust your environment variables, notably PATH, to include the Go `bin` directory so that when you are at the Terminal/Command Prompt/PowerShell/shell and you enter `go`, it is available.
3. Install the Fyne.io toolkit prerequisites.  From [here](https://developer.fyne.io/started/),
> Fyne requires 3 basic elements to be present, the Go tools (at least version 1.12), a C compiler (to connect with system graphics drivers) and an system graphics driver.

Now I am on macOS and already have XCode installed.  YMMV.  But follow that last link for more details to make sure you have the bits installed right.

Once you have that done, the key thing is to install the actual Fyne.io toolkit.  This typically means entering the command
```
go get fyne.io/fyne/v2
```

But for completeness, in case you should want to develop your own Go/Fyne.io based GUI applications, the process for new Fyne apps goes something like this:
```
# Create directory for your app (myapp)
mkdir myapp
# Change into the directory
cd myapp
# Initialize your Go project
go mod init myapp
# Install the Fyne.io toolkit
go get fyne.io/fyne/v2
# Edit Go code in your favorite editor/IDE (I strongly recommend VSCode with the Go extension)
code .
```
Here you might then write a simple app such as this:
```
package main

import (
    "fyne.io/fyne/v2/app"
    "fyne.io/fyne/v2/widget"
)

func main() {
    myapp := app.New()
    window := myapp.NewWindow("MyApp")
    window.SetContent(widget.NewLabel("This is MyApp"))
    window.ShowAndRun()
}
```
Once you save that, it's Go time.  (Sorry.  Couldn't resist.)
```
# Any time you need to clean up the config
go mod tidy
# To just run your program on the fly
go run .
# To run it with a specific theme
FYNE_THEME=dark go run .
# To compile it to 'myapp' binary (or 'myapp.exe' in Windows)
go build .
```

# Packaging Fyne apps for the Desktop

When you are ready to truly build a desktop application for distribution, Fyne has you covered.  You can read more about it here:  https://developer.fyne.io/started/packaging

But simply put, install the `fyne` command line tool using
```
go install fyne.io/fyne/v2/cmd/fyne@latest
```

Now you can do something as simple as the following:

1. Place an icon file named `Icon.png` in the same directory with your Go/Fyne source files.
2. Run the command
```
fyne package
```
and you will have an application for your host OS.  In the case of macOS, `fyne` builds a proper application bundle.  For Windows, it builds a proper `.exe.zip` for distribution.


# Cross Compiling to other OSes

Go has the ability to compile binaries for platforms other than the host OS you are running it on.  This typically takes the form of modifying the environment variables `GOOS` (darwin, linux, windows, android, ios) and `GOARCH` (amd64, arm64).  For example, I am on an Intel x64-based Mac running macOS.  If I wanted to compile a Go program to a Windows .exe, I could use
```
GOOS=windows GOARCH=amd64 go build .
```
and the Go compiler would build a Windows 64-bit .exe.

Fyne.io, being written in Go, turns out to also be able to be cross-compiled as well.

HOWEVER, this can be a little tricky.  For example, again, I am on macOS.  As Fyne.io needs access to a graphics driver for Windows, if I try using the `fyne` package directly with a command such as
```
fyne package -os windows -icon myicon.png
```
it fails due to not having everything it needs.

## fyne-cross

Now not to fret.  The fine folks (sorry, did it again) at Fyne have also created a very handy program called [`fyne-cross`](https://developer.fyne.io/started/cross-compiling.html) that is just one more command away:
```
go get github.com/fyne-io/fyne-cross
```
Now `fyne-cross` does require that you have Docker installed and running, as it leverages Docker containers to do its work.  (If you are on a Linux host doing your Go/Fyne work, you may not need this tool at all.  They clearly leverage Docker as it provides a Linux environment for doing the heavy lifting.)

But if you have all the bits in place, you can now, from a single host (in my case my macOS system), build binaries for all the major platforms.  To do so, you use commands such as
```
fyne-cross windows -name MyApp.exe -app-version 0.0.1 -app-build 1 -app-id com.example.MyApp -icon MyIcon.png -release
```

# macOS Universal Binaries

This one is OS-specific, so if you are not interested in macOS, just skip it.  For those who are using Macs, you may know that Apple sells Macs with two different chip architectures in them:  Intel x64 chips and Apple Silicon chips such as the M1, M1 Pro, M1 Max, and M1 Ultra.

Now Go compiles code down to a specific OS _and_ architecture.  This means if, like me, you are on an x64-based Mac, the Go compiler will default to building "Intel based" macOS binaries.  I can adjust the `GOARCH` environment variable to be `arm64`, but then Go creates an "Apple Silicon" macOS binary that I can't run at all.

Go does not, natively, have the ability to create what are called "fat binaries", or Universal Binaries in the macOS world.  However, there is a tool in the macOS toolchain called `lipo` which can take care of this for you.  That is, you can build 2 versions of a Go program--one x64 and the other ARM64--and then use `lipo` to mash them together into a single Universal Binary.

## One More Thing...

Since we are using `fyne` to package up our Go/Fyne applications, when working on the macOS versions, we end up not with just a single binary file of each architecture, but rather an application bundle of each.  So how to actually build a Universal Binary version of a macOS application where we have 2 distinct macOS application bundles?

Well, it's actually not that hard.  First, we need to understand what exactly an application bundle entails:

### Directory Structure of a Fyne.io macOS application
```
├── MyApp.app
│   └── Contents
│       ├── Info.plist      <== This contains info such as version, build, etc.
│       ├── MacOS
│       │   └── MyApp       <== This is the Go binary
│       └── Resources
│           └── icon.icns   <== This is the icon file

```
So we build our architecture-specific application bundles and name each something like `MyApp_amd64.app` and `MyApp_arm64.app`.  We then copy one of those application bundles over to a final `MyApp.app`, which gives us the directory structure intact.  Then we use `lipo` against the _actual_, architecture-specific Go binaries located down in each of the other bundles, create a Universal Binary, and place that in the final application's `MacOS` subdirectory.

And, in fact, if you look at the file `buildapp.sh.example` provided in this repo, you can see how this is done.  You can copy that file, adjust the variables near the top as needed, and if you are on a Mac, run this and it will build you both a Universal Binary macOS application bundle stored in a `.DMG` and a Windows 64-bit application stored in a `.exe.zip`, all for easy distribution.


# FINAL NOTES

## fyne-cross

### Using fyne-cross on macOS Host to build Windows application
```
fyne-cross windows -name MyApp.exe -app-version 0.0.1 -app-build 1 -app-id com.example.MyApp -icon MyIcon.png
```

## fyne

### Using fyne on Windows (and possibly Linux) to build Windows applications
```
GOOS=windows GOARCH=amd64 fyne package --name MyApp --appVersion 0.0.1 --appBuild 1 --appID com.example.MyApp --icon MyIcon.png --release
```

### Using fyne on macOS to build architecture-specific macOS application bundles
```
GOOS=darwin GOARCH=amd64 fyne package --name MyApp_amd64 --appVersion 0.0.1 --appBuild 1 --appID com.example.MyApp  --icon MyIcon.png --release
GOOS=darwin GOARCH=arm64 fyne package --name MyApp_arm64 --appVersion 0.0.1 --appBuild 1 --appID com.example.MyApp  --icon MyIcon.png --release
```

### Copy Intel version to final name
```
cp -R MyApp_amd64.app/ MyApp.app/
```

### Combine Intel and Apple Silicon binaries into a Universal Binary which is output to this final copy
```
lipo -create -output ./MyApp.app/Contents/MacOS/MyApp ./MyApp_amd64.app/Contents/MacOS/MyApp ./MyApp_arm64.app/Contents/MacOS/MyApp
```