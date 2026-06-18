#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Go代码静态验证脚本
检查Go源文件的语法结构、括号匹配、导入完整性
"""

import os
import re
import sys
from pathlib import Path

BACKEND_DIR = Path(__file__).parent
GO_FILES = list(BACKEND_DIR.rglob("*.go"))

def check_brackets(content, filepath=None):
    """检查括号匹配"""
    stack = []
    bracket_map = {')': '(', ']': '[', '}': '{'}
    lines = content.split('\n')
    
    for line_num, line in enumerate(lines, 1):
        in_string = False
        in_comment = False
        string_char = ''
        
        for col, char in enumerate(line, 1):
            if in_comment:
                if char == '/' and col > 1 and line[col-2] == '*':
                    in_comment = False
                continue
            
            if in_string:
                if char == string_char and line[col-2] != '\\':
                    in_string = False
                continue
            
            if char in ('"', "'"):
                in_string = True
                string_char = char
                continue
            
            if col < len(line) and char == '/' and line[col] == '*':
                in_comment = True
                continue
            
            if char in '([{':
                stack.append((char, line_num, col))
            elif char in ')]}':
                if not stack:
                    return False, f"行 {line_num}, 列 {col}: 多余的 '{char}'"
                last_char, last_line, last_col = stack.pop()
                if last_char != bracket_map[char]:
                    return False, f"行 {line_num}, 列 {col}: '{char}' 与行 {last_line}, 列 {last_col} 的 '{last_char}' 不匹配"
    
    if stack:
        char, line, col = stack[0]
        return False, f"行 {line}, 列 {col}: 未闭合的 '{char}'"
    
    return True, "OK"

def check_imports(content, filepath):
    """检查导入语句是否有效"""
    import_pattern = r'import\s*\(([^)]*)\)'
    match = re.search(import_pattern, content, re.DOTALL)
    
    if not match:
        single_import = re.search(r'import\s+"[^"]+"', content)
        if not single_import and 'import' in content:
            return False, "import 语句格式错误"
        return True, "OK"
    
    import_block = match.group(1)
    lines = import_block.strip().split('\n')
    
    for line in lines:
        line = line.strip()
        if not line or line.startswith('//'):
            continue
        if not re.match(r'^"[^"]+"$', line) and not re.match(r'^\w+\s+"[^"]+"$', line):
            if not line.startswith('"') and not (line and line[0].isalpha()):
                return False, f"导入语句格式错误: {line}"
    
    return True, "OK"

def check_func_signatures(content, filepath):
    """检查函数签名格式"""
    func_pattern = r'func\s+(\([^)]*\)\s+)?(\w+)\s*\('
    funcs = re.finditer(func_pattern, content)
    
    for func_match in funcs:
        func_name = func_match.group(2)
        if not func_name[0].isupper() and not func_name.startswith('_'):
            if func_name in ('main', 'init'):
                continue
    
    return True, "OK"

def check_package(content, filepath):
    """检查package声明"""
    first_line = content.lstrip().split('\n')[0]
    if not first_line.startswith('package '):
        return False, "缺少package声明"
    return True, "OK"

def check_struct_tags(content, filepath):
    """检查结构体标签格式"""
    tag_pattern = r'`([^`]+)`'
    tags = re.finditer(tag_pattern, content)
    
    for tag_match in tags:
        tag = tag_match.group(1)
        if not re.match(r'^(json|db|mapstructure|form):"[^"]+"(\s+\w+:"[^"]+")*$', tag):
            if 'json' in tag or 'db' in tag or 'mapstructure' in tag:
                if ':"' not in tag:
                    return False, f"标签格式可能错误: {tag}"
    
    return True, "OK"

def verify_go_file(filepath):
    """验证单个Go文件"""
    filepath = Path(filepath)
    rel_path = filepath.relative_to(BACKEND_DIR)
    
    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()
    
    errors = []
    
    checks = [
        ("Package", check_package),
        ("Brackets", check_brackets),
        ("Imports", check_imports),
        ("Functions", check_func_signatures),
        ("Struct Tags", check_struct_tags),
    ]
    
    for check_name, check_func in checks:
        try:
            ok, msg = check_func(content, filepath)
            if not ok:
                errors.append(f"[{check_name}] {msg}")
        except Exception as e:
            errors.append(f"[{check_name}] 检查异常: {str(e)}")
    
    return rel_path, errors

def main():
    print(f"检查目录: {BACKEND_DIR}")
    print(f"找到 {len(GO_FILES)} 个Go文件\n")
    
    total_errors = 0
    for go_file in GO_FILES:
        rel_path, errors = verify_go_file(go_file)
        
        if errors:
            print(f"❌ {rel_path}")
            for err in errors:
                print(f"   {err}")
            total_errors += len(errors)
        else:
            print(f"✅ {rel_path}")
    
    print(f"\n总计: {len(GO_FILES)} 个文件, {total_errors} 个问题")
    
    return 0 if total_errors == 0 else 1

if __name__ == '__main__':
    sys.exit(main())
