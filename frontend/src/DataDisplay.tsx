export const DataDisplay = (props: {data: string[]}) => {
    return (
        <div>
            <h2>Data Display</h2>
            <ul>
                {props.data.map((item, index) => (
                    <li key={index}>{item}</li>
                ))}
            </ul>
        </div>
    )
}