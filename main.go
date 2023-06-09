package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// go run main.go -dir="/Users/yusuke.konishi/work/github.com/Kyash/platform-api"

func main() {
	// コマンドライン引数からディレクトリのパスを取得
	dirPath := flag.String("dir", "", "directory path")
	flag.Parse()

	if *dirPath == "" {
		log.Fatal("Directory path is required")
	}

	// ディレクトリ内のGoファイルのパスを再帰的に取得
	goFiles, err := findGoFiles(*dirPath)
	if err != nil {
		log.Fatal("Error finding Go files:", err)
	}

	// 文字列定数を取得
	constants, err := extractStringConstants(goFiles)
	if err != nil {
		log.Fatal("Error extracting string constants:", err)
	}

	// 取得した文字列定数を表示
	for _, constant := range constants {
		fmt.Println(constant)
	}
}

// 指定したディレクトリから再帰的にGoファイルのパスを取得する関数
func findGoFiles(dirPath string) ([]string, error) {
	var goFiles []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(info.Name(), ".go") {
			goFiles = append(goFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return goFiles, nil
}

// 指定したGoファイル群から文字列定数を抽出する関数
func extractStringConstants(filePaths []string) ([]string, error) {
	// 文字列定数を格納するスライス
	constants := make([]string, 0)

	// WaitGroupを使用して並行処理を管理
	var wg sync.WaitGroup
	wg.Add(len(filePaths))

	// ファイルごとに解析（並行処理）
	for _, filePath := range filePaths {
		go func(path string) {
			defer wg.Done()

			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				log.Println("Error parsing file:", err)
				return
			}

			// 定数宣言の検索
			ast.Inspect(node, func(n ast.Node) bool {
				// 定数宣言ノードかどうかをチェック
				decl, ok := n.(*ast.GenDecl)
				if !ok || decl.Tok != token.CONST {
					return true
				}

				// 定数の詳細を検査
				for _, spec := range decl.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}

					// 定数の値が文字列定数であるかをチェック
					for _, value := range valueSpec.Values {
						basicLit, ok := value.(*ast.BasicLit)
						if !ok || basicLit.Kind != token.STRING {
							continue
						}

						// 文字列定数をスライスに追加
						constants = append(constants, basicLit.Value)
					}
				}

				return true
			})
		}(filePath)
	}

	// すべての解析が完了するまで待機
	wg.Wait()

	return constants, nil
}
