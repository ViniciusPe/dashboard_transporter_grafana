import { AppPlugin } from '@grafana/data';
import { ImportPage } from './pages/ImportPage';

export const plugin = new AppPlugin().setRootPage(ImportPage);
