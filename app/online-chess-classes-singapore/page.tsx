import LocationPageTemplate, { getLocationMetadata } from '../components/LocationPageTemplate';
import { Metadata } from 'next';

export const metadata: Metadata = getLocationMetadata('singapore');

export default function Page() {
  return <LocationPageTemplate cityId="singapore" />;
}
