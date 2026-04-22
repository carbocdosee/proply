import { addMessages, init, getLocaleFromNavigator, locale } from 'svelte-i18n';
import en from './en.json';
import ru from './ru.json';

export { locale };

let initialized = false;

export function setupI18n(initialLocale = 'en') {
	if (!initialized) {
		addMessages('en', en);
		addMessages('ru', ru);
		init({
			fallbackLocale: 'en',
			initialLocale: (initialLocale || getLocaleFromNavigator() || 'en').substring(0, 2)
		});
		initialized = true;
	} else {
		locale.set(initialLocale || 'en');
	}
}

export function setLocale(lang: string) {
	locale.set(lang || 'en');
}
