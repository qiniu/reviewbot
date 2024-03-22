# Cppcheck - A tool for static C/C++ code analysis
# cppcheck linter
Syntax:<br />
    cppcheck [OPTIONS] [files or paths]<br />

in this repo:<br />
Recursively check the current folder, format the error messages as file name:line number:column number: warning message and don't print progress:<br />
cppcheck --quiet --template='{file}:{line}:{column}:  {message}' .<br />

For more information:<br />
    https://files.cppchecksolutions.com/manual.pdf<br />
