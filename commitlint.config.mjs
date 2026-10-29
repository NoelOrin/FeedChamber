export default {
	extends: ['@commitlint/config-conventional'],
	rules: {
		'body-leading-blank': [2, 'always'],
		'type-empty': [2, 'never'],
		'subject-case': [0],
		'type-enum': [
			2,
			'always',
			['feat', 'fix', 'docs', 'style', 'refactor', 'perf', 'test', 'chore', 'revert', 'build', 'ci', 'types', 'wip'],
		],
	},
};
