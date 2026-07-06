import typescriptEslint from '@typescript-eslint/eslint-plugin'
import tsParser from '@typescript-eslint/parser'
import prettierConfig from 'eslint-config-prettier'
import PluginImport from 'eslint-plugin-import'
import eslintPluginPrettier from 'eslint-plugin-prettier'
import reactPkg from 'eslint-plugin-react'
import reactHooksPlugin from 'eslint-plugin-react-hooks'
import simpleImportSort from 'eslint-plugin-simple-import-sort'

const reactPlugin = reactPkg.default || reactPkg
const reactConfigs = reactPkg.configs

const eslintConfig = [
    {
        ignores: ['node_modules/**', 'dist/**', '**/*.d.ts', '**/*.config.js', '**/*.config.mjs', '**/*.config.ts'],
    },
    {
        files: ['src/**/*.{js,jsx,ts,tsx}'],

        plugins: {
            react: reactPlugin,
            import: PluginImport,
            prettier: eslintPluginPrettier,
            'react-hooks': reactHooksPlugin,
            '@typescript-eslint': typescriptEslint,
            'simple-import-sort': simpleImportSort,
        },

        settings: {
            react: {
                version: 'detect',
            },
        },

        languageOptions: {
            parser: tsParser,
            ecmaVersion: 2022,
            sourceType: 'module',
            parserOptions: {
                project: './tsconfig.json',
            },
        },

        rules: {
            // ===============================
            // React
            // ===============================
            ...reactConfigs.recommended.rules,
            'react/react-in-jsx-scope': 'off',
            'react/jsx-uses-react': 'off',
            'react/jsx-key': 'error',
            'react/prop-types': 'off',
            'react/jsx-curly-spacing': ['error', { when: 'always', children: true }],
            'react/jsx-tag-spacing': ['error', { closingSlash: 'never', beforeSelfClosing: 'always' }],
            'react/jsx-fragments': ['error', 'syntax'],
            'react-hooks/rules-of-hooks': 'error',
            'react-hooks/exhaustive-deps': 'warn',

            // ===============================
            // TypeScript
            // ===============================
            'no-unused-vars': 'off',
            '@typescript-eslint/ban-ts-comment': 'error',
            '@typescript-eslint/default-param-last': 'error',
            '@typescript-eslint/no-unused-vars': [
                'error',
                { args: 'after-used', argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
            ],
            '@typescript-eslint/consistent-type-definitions': ['error', 'interface'],
            '@typescript-eslint/no-floating-promises': ['warn', { ignoreVoid: false }],
            '@typescript-eslint/no-misused-promises': ['error', { checksVoidReturn: false }],
            '@typescript-eslint/explicit-module-boundary-types': 'off',
            '@typescript-eslint/no-explicit-any': ['error', { fixToUnknown: true, ignoreRestArgs: false }],

            // ===============================
            // Import
            // ===============================
            'import/no-unresolved': 'off',
            'import/prefer-default-export': 'off',
            'import/no-cycle': ['warn', { maxDepth: 1 }],
            'simple-import-sort/exports': 'error',
            'simple-import-sort/imports': [
                'error',
                {
                    groups: [['^\\u0000', '^[a-z]'], ['^@'], ['^\\.'], ['^.+\\.css$']],
                },
            ],

            // ===============================
            // General
            // ===============================
            'array-callback-return': 'error',
            'brace-style': ['error', '1tbs', { allowSingleLine: true }],
            camelcase: 'warn',
            'dot-notation': 'error',
            'eol-last': ['error', 'always'],
            eqeqeq: 'error',
            indent: 'off',
            'max-depth': ['error', 5],
            'no-nested-ternary': 'error',
            'operator-linebreak': ['error', 'after'],
            'no-multiple-empty-lines': ['error', { max: 1, maxEOF: 1 }],
            'object-curly-spacing': ['error', 'always'],
            'no-console': ['error', { allow: ['warn', 'error'] }],

            // ===============================
            // Prettier
            // ===============================
            'prettier/prettier': 'error',
            ...prettierConfig.rules,
        },
    },
]

export default eslintConfig
