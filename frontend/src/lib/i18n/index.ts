import { addMessages, init, getLocaleFromNavigator } from 'svelte-i18n';
import en from './en.json';
import ru from './ru.json';

export function setupI18n(locale = 'en') {
	addMessages('en', en);
	addMessages('ru', ru);

	init({
		fallbackLocale: 'en',
		initialLocale: locale || getLocaleFromNavigator() || 'en'
	});
}
