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
	"unicode/utf8"
)

// 引数で渡されたパス内のgoファイルの中から文字列定数と該当ファイル行数を出力する
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

	// 取得した文字列定数を出力
	for _, constant := range constants {
		fmt.Println(constant)
	}
}

// 指定したディレクトリから再帰的にGoファイルのパスを取得する
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

// 指定したGoファイル群から文字列定数を抽出する
func extractStringConstants(filePaths []string) ([]string, error) {
	// 文字列定数と該当箇所を格納する構造体のスライス
	constantDetails := make([]struct {
		Constant string
		FilePath string
		Line     int
	}, 0)

	// WaitGroupを使用して並行処理を管理
	var wg sync.WaitGroup
	wg.Add(len(filePaths))

	// ファイルごとに解析
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

						// 非ASCII文字が含まれているかチェック
						hasNonASCII := false
						for _, r := range basicLit.Value {
							if r >= utf8.RuneSelf {
								hasNonASCII = true
								break
							}
						}

						if hasNonASCII {
							// 文字列定数と該当箇所をスライスに追加
							constantDetails = append(constantDetails, struct {
								Constant string
								FilePath string
								Line     int
							}{
								Constant: basicLit.Value,
								FilePath: path,
								Line:     fset.Position(basicLit.Pos()).Line,
							})
						}
					}
				}

				return true
			})

		}(filePath)
	}

	// すべての解析が完了するまで待機
	wg.Wait()

	// 文字列定数と該当箇所を表示
	constantLines := make([]string, len(constantDetails))
	for i, detail := range constantDetails {
		constantLines[i] = fmt.Sprintf("%s:%d: %s", detail.FilePath, detail.Line, detail.Constant)
	}

	return constantLines, nil
}
