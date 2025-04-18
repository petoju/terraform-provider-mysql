
set PATH=C:\cygwin64\bin;%PATH%
cd test1
if errorlevel 1 goto exit
C:\cygwin64\bin\bash.exe ../test.sh
if errorlevel 1 goto exit 
cd ..
if errorlevel 1 goto exit
cd test2
if errorlevel 1 goto exit
C:\cygwin64\bin\bash.exe ../test.sh
:exit



