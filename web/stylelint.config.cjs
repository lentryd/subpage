/** @type {import('stylelint').Config} */
const stylelintConfig = {
    plugins: ['stylelint-prettier', 'stylelint-selector-tag-no-without-class'],
    extends: ['stylelint-config-standard'],
    rules: {
        'prettier/prettier': true,
        'color-no-invalid-hex': true,
        'unit-no-unknown': true,
        'function-no-unknown': true,
        'no-unknown-animations': true,
        'no-descending-specificity': null,
        'property-no-vendor-prefix': null,
    },
    overrides: [
        {
            files: ['**/*.module.*'],
            rules: {
                'selector-class-pattern': [
                    '^(?!.*(__|--))[a-z][a-zA-Z0-9]*$',
                    {
                        message: 'Classes must be in camelCase',
                        resolveNestedSelectors: true,
                    },
                ],
                'plugin/selector-tag-no-without-class': [
                    ['div', 'span'],
                    {
                        message: 'Tag selectors are not allowed, use only class selectors',
                    },
                ],
            },
        },
    ],
}

module.exports = stylelintConfig
