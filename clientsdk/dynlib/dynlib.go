package dynlib
import "fmt"
// DynamicLibrary 是动态库的通用接口
type DynamicLibrary interface {
	Call(funcName string, args ...interface{}) (uintptr, uintptr, error)
	Close() error
}

// baseLibrary 是通用的动态库实现
type baseLibrary struct {
	handle uintptr
}

// NewLibrary 根据平台加载动态库
func NewLibrary(path string) (DynamicLibrary, error) {
	fmt.Println("正在尝试加载库，路径为:", path) // 必须看到这个输出
	return newLibrary(path)
}

// newLibrary 是平台特定的实现
var newLibrary func(path string) (DynamicLibrary, error)
